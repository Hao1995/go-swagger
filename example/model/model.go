package model

import "gitlab.paradise-soft.com.tw/backend/goas/example/paging"

type Data struct {
	Aparam string        `json:"Aparam"`
	Data   paging.Paging `json:"data"`
}

// type Data2 struct {
// 	Name string `json:"name"`
// }
