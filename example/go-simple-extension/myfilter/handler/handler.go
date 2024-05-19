package handler

import "time"

type foo struct {
	Name string
}

type localdata struct {
	Name string
	Foo  *foo
}

type globaldata struct {
	Name  string
	Age   int
	Time  time.Time
	Time2 time.Time
	Time3 time.Time
	Foo   *foo
}

type dataKey string

const (
	GLOBAL = dataKey("GLOBAL")
)
