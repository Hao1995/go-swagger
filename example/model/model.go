package model

import "gitlab.paradise-soft.com.tw/backend/goas/example/paging"

type Data struct {
	Data paging.Paging `json:"data"`
}

// type Data2 struct {
// 	Name string `json:"name"`
// }
