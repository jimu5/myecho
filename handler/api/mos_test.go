package api

import (
	"bytes"
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"testing"

	"github.com/gofiber/fiber/v2"

	"myecho/dal/connect"
	"myecho/dal/mysql"
	"myecho/model"
	"myecho/service"
	"myecho/utils"
)

func TestFileUploadMetadataOwnershipAndRemotePath(t *testing.T) {
	setupAPITestDB(t)
	chdirToTemp(t)
	author := &model.User{BaseModel: model.BaseModel{ID: 41}, PermissionType: model.Author}
	app := fiber.New()
	withAuthor := func(next fiber.Handler) fiber.Handler {
		return func(c *fiber.Ctx) error {
			c.Locals("user", author)
			return next(c)
		}
	}
	app.Post("/files/upload", withAuthor(FileSingleUpload))
	app.Post("/save_url_file", withAuthor(FileSaveByLinkUrl))
	app.Get("/files", withAuthor(FilePageList))
	app.Put("/files/:id", withAuthor(FileInfoUpdate))

	localPNG := testPNGBytes(t, 640, 320)
	body, contentType := multipartImageBody(t, "file", "local.png", localPNG)
	req := httptest.NewRequest(fiber.MethodPost, "/files/upload", body)
	req.Header.Set(fiber.HeaderContentType, contentType)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("local upload app.Test() error = %v", err)
	}
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("local upload status = %d, want 200", resp.StatusCode)
	}
	var uploaded service.File
	if err := json.Unmarshal(decodeAPIResp(t, resp).Data, &uploaded); err != nil {
		t.Fatalf("decode local upload: %v", err)
	}
	if uploaded.UploaderID != author.ID || uploaded.Width != 640 || uploaded.Height != 320 || uploaded.ThumbnailURL == "" {
		t.Fatalf("local upload metadata = %+v", uploaded)
	}
	var localModel mysql.FileModel
	if err := connect.Database.First(&localModel, uploaded.ID).Error; err != nil {
		t.Fatalf("load local file: %v", err)
	}
	if _, err := os.Stat(localModel.GetThumbnailSavePath()); err != nil {
		t.Fatalf("local thumbnail stat: %v", err)
	}

	remotePNG := testPNGBytes(t, 100, 50)
	oldRemoteClient := utils.RemoteFileHTTPClient
	utils.RemoteFileHTTPClient = &http.Client{Transport: apiRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode:    http.StatusOK,
			Status:        "200 OK",
			Body:          io.NopCloser(bytes.NewReader(remotePNG)),
			ContentLength: int64(len(remotePNG)),
			Header:        make(http.Header),
			Request:       req,
		}, nil
	})}
	t.Cleanup(func() { utils.RemoteFileHTTPClient = oldRemoteClient })
	resp = doJSONRequest(t, app, fiber.MethodPost, "/save_url_file", "", `{"url":"http://93.184.216.34/remote.png"}`)
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("remote upload status = %d, want 200", resp.StatusCode)
	}
	var remoteModel mysql.FileModel
	if err := connect.Database.Where("name = ?", "remote").First(&remoteModel).Error; err != nil {
		t.Fatalf("load remote file: %v", err)
	}
	if remoteModel.UploaderID != author.ID || remoteModel.Width != 100 || remoteModel.Height != 50 || remoteModel.ThumbnailURL == "" {
		t.Fatalf("remote upload metadata = %+v", remoteModel)
	}

	other := &mysql.FileModel{Name: "other", ExtensionName: ".txt", DirPath: localModel.DirPath, UUID: "other-file", MD5: "other-md5", UploaderID: 99}
	if err := (&mysql.FileRepo{}).Create(other); err != nil {
		t.Fatalf("create other file: %v", err)
	}
	legacy := &mysql.FileModel{Name: "legacy", ExtensionName: ".txt", DirPath: localModel.DirPath, UUID: "legacy-file", MD5: "legacy-md5"}
	if err := (&mysql.FileRepo{}).Create(legacy); err != nil {
		t.Fatalf("create legacy file: %v", err)
	}
	resp = doJSONRequest(t, app, fiber.MethodGet, "/files", "", "")
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("author list status = %d, want 200", resp.StatusCode)
	}
	var listed []service.File
	if err := json.Unmarshal(decodeAPIResp(t, resp).Data, &listed); err != nil {
		t.Fatalf("decode author list: %v", err)
	}
	if len(listed) != 2 {
		t.Fatalf("author list len = %d, want 2", len(listed))
	}
	resp = doJSONRequest(t, app, fiber.MethodPut, "/files/"+strconv.FormatUint(uint64(other.ID), 10), "", `{"full_name":"other.txt","alt_text":"blocked"}`)
	if resp.StatusCode != fiber.StatusForbidden {
		t.Fatalf("author update other status = %d, want 403", resp.StatusCode)
	}
}

func testPNGBytes(t *testing.T, width, height int) []byte {
	t.Helper()
	imageData := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			imageData.Set(x, y, color.RGBA{R: uint8(x), G: uint8(y), B: 200, A: 255})
		}
	}
	var data bytes.Buffer
	if err := png.Encode(&data, imageData); err != nil {
		t.Fatalf("encode test PNG: %v", err)
	}
	return data.Bytes()
}

func multipartImageBody(t *testing.T, field, filename string, data []byte) (*bytes.Buffer, string) {
	t.Helper()
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile(field, filename)
	if err != nil {
		t.Fatalf("CreateFormFile() error = %v", err)
	}
	if _, err := part.Write(data); err != nil {
		t.Fatalf("write multipart image: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close multipart writer: %v", err)
	}
	return body, writer.FormDataContentType()
}
