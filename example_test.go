package upstashdis_test

import (
	"fmt"
	"log"
	"os"

	"github.com/mna/upstashdis"
)

func ExampleClient() {
	client := upstashdis.Client{
		BaseURL:  os.Getenv("UPSTASH_REST_URL"),
		APIToken: os.Getenv("UPSTASH_REST_TOKEN"),
	}

	req := client.NewRequest()
	if err := req.Send("SET", "mykey", 1); err != nil {
		log.Fatal(err)
	}
	if err := req.Send("INCR", "mykey"); err != nil {
		log.Fatal(err)
	}
	var setRes string
	var incrRes int
	if err := req.Exec(&setRes, &incrRes); err != nil {
		log.Fatal(err)
	}
	fmt.Println(setRes, incrRes) // OK, 2
}
