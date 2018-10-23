package model

import "github.com/Hao1995/go-swagger/example/paging"

type Data struct {
	Aparam string        `json:"Aparam"`
	Data   paging.Paging `json:"data"`
}

// type Data2 struct {
// 	Name string `json:"name"`
// }
