package main

import (
	"github.com/linxlib/pbar"
	"os"
	"strconv"
	"time"
)

func main() {
	count := -1
	bar1 := pbar.AddBar(count)
	bar1.SetTemplate(pbar.InkWalkTemplate)

	pbar.Start()

	go func() {
		for i := 1; i <= 1000; i++ {
			bar1.Inc()
			if i%4 == 0 {
				bar1.Set("doing", "doing "+strconv.Itoa(i))
			}
			time.Sleep(time.Millisecond * 10)
		}
		bar1.Set("success", "OK")
		bar1.Finish()
	}()

	//bar2 := pbar.AddBar(count)
	//pbar.Start()
	//
	//go func() {
	//
	//	for i := 1; i <= count; i++ {
	//		bar2.Inc()
	//		time.Sleep(time.Millisecond * 20)
	//	}
	//	bar2.Set("fail", "×")
	//	//bar2.FinishAll()
	//}()

	time.Sleep(time.Second * 20)
	pbar.FinishAll()

	time.Sleep(time.Second * 2)
	os.Exit(1)

}
