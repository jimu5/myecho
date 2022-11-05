package rtype

import "myecho/dal/mysql"

type LinkQueryParam struct {
	CategoryUID *string `query:"category_uid"`
}

func (lqp *LinkQueryParam) ToDALParam() mysql.LinkCommonQueryParam {
	return mysql.LinkCommonQueryParam{
		CategoryUID: lqp.CategoryUID,
	}
}
