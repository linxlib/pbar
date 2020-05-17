package main

import (
	"github.com/linxlib/pbar"
	"os"
	"time"
)

func main() {
	count := 100
	bar1 := pbar.AddBar(count)

	pbar.Start()

	go func() {
		for i := 1; i <= count; i++ {
			bar1.Inc()
			time.Sleep(time.Millisecond * 10)
		}
		bar1.Set("success", "√")
		bar1.Finish()
	}()

	bar2 := pbar.AddBar(count)
	pbar.Start()

	go func() {

		for i := 1; i <= count; i++ {
			bar2.Inc()
			time.Sleep(time.Millisecond * 20)
		}
		bar2.Set("fail", "×")
		//bar2.FinishAll()
	}()

	time.Sleep(time.Second * 5)
	pbar.FinishAll()

	time.Sleep(time.Second * 2)
	os.Exit(1)

}
