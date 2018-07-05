package core

import "time"

type String struct {
	string
	Valid bool
}

type DateTime struct {
	time.Time
	Valid bool
}
