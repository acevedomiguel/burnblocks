package openrouter_test

import (
	"context"
	"fmt"

	openrouter "github.com/acevedomiguel/burnblocks/openrouter"
)

func ExampleFetch() {
	snap, err := openrouter.Fetch(context.Background(), fakeAPI{}, openrouter.FetchOptions{Now: fixedTime()})
	if err != nil {
		panic(err)
	}
	fmt.Println(snap.Account.Remaining)
	// Output: 42.1
}
