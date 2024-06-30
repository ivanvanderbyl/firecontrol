package main

import (
	"fmt"

	"github.com/ivanvanderbyl/escea-fireplace/pkg/firecontrol"
)

func main() {
	fs, err := firecontrol.SearchForFireplaces()
	if err != nil {
		panic(err)
	}

	for _, f := range fs {
		fmt.Println("Found Fireplace", f.PIN, f.Serial)
	}
}
