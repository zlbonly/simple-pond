package main

import (
	"fmt"
	"simple-pond/pond"
	"time"
)

func main()  {

	pool ,err := pond.NewPool(10)
	if err != nil{
		panic(err)
	}

	for i :=0 ;i< 20;i++ {
		pool.Put(&pond.Task{
			Handler: func(v ...interface{}) {
				fmt.Println(v)
			},
			Params:  []interface{}{i},
		})
	}
	time.Sleep(1e9)
}


