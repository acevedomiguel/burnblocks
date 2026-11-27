package deepseek_test

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/acevedomiguel/burnblocks/deepseek"
)

func ExampleFetch() {
	reading := deepseek.BalanceReading{
		ObservedAt: time.Date(2026, 11, 27, 9, 30, 0, 0, time.UTC),
		Available:  deepseek.AvailabilityAvailable,
		Balances: []deepseek.Balance{
			{Currency: deepseek.CurrencyCNY, Total: ptr(123.45)},
		},
	}
	snap, err := deepseek.Fetch(context.Background(), fakeSource{reading: reading}, deepseek.FetchOptions{})
	if err != nil {
		fmt.Println(err)
		return
	}

	var out bytes.Buffer
	if err := deepseek.WriteJSON(&out, snap); err != nil {
		fmt.Println(err)
		return
	}

	fmt.Println("schema:", snap.SchemaVersion)
	fmt.Println("currency:", snap.Balances[0].Currency)
	fmt.Println("runway:", snap.Runway.Status)
	fmt.Println("has total:", strings.Contains(out.String(), `"total": 123.45`))

	// Output:
	// schema: 1
	// currency: CNY
	// runway: unavailable
	// has total: true
}
