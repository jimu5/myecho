package utils

import (
	"math/rand"
	"strconv"
	"time"
)

func getRandomString(n int) string {
	str := "0123456789abcdefghijklmnopqrstuvwxyz"
	bytes := []byte(str)
	var result []byte
	for i := 0; i < n; i++ {
		result = append(result, bytes[rand.Intn(len(bytes))])
	}
	return string(result)
}

func GenUID20() string {
	t := time.Now().UnixMicro()      // ms 16位
	tStr := strconv.FormatInt(t, 16) // 13位
	return getRandomString(7) + tStr
}
