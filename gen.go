//go:build ignore

// Copyright 2019, 2024 Tamás Gulácsi. All rights reserved.
//
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"go/format"
	"log"
	"maps"
	"os"
	"os/signal"
	"slices"
	"strings"
	"time"

	"github.com/UNO-SOFT/bankinap"
	"github.com/UNO-SOFT/bankinap/download"

	"github.com/google/renameio/v2"
)

func main() {
	if err := Main(); err != nil {
		log.Fatal(err)
	}
}

func Main() error {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	bux, err := download.SearchBUX(ctx, "")
	if err != nil {
		return err
	}
	holidays := make(map[bankinap.Date]bankinap.BUXHoliday, len(bux))
	for _, h := range bux {
		holidays[h.Date] = h
	}

	yy, err := download.SearchXLSXURL(ctx, "")
	if err != nil {
		return err
	}
	dis, err := os.ReadDir(".")
	if len(dis) == 0 && err != nil {
		return err
	}
	have := make(map[uint16]bankinap.YearDays, len(dis))
	for _, di := range dis {
		if !strings.HasSuffix(di.Name(), ".json") {
			continue
		}
		b, err := os.ReadFile(di.Name())
		if err != nil {
			log.Printf("read %q: %+v", di.Name(), err)
			continue
		}
		var year bankinap.YearDays
		if err := json.Unmarshal(b, &year); err != nil || len(year.Days) < 365 {
			log.Printf("unmarshal %q: %+v (%d)", di.Name(), err, len(year.Days))
			continue
		}
		for _, d := range year.Days {
			if _, ok := holidays[d.Date]; ok {
				d.BUX = true
			}
		}
		have[year.Year] = year
	}

	for _, y := range yy {
		if _, ok := have[y.Year]; ok {
			continue
		}
		Y, err := download.Get(ctx, y)
		if err != nil {
			log.Printf("ERROR Get(%v): %+v", y, err)
			return err
		}
		have[Y.Year] = Y
	}

	for _, Y := range have {
		for i, d := range Y.Days {
			if _, ok := holidays[d.Date]; ok {
				d.BUX = true
				Y.Days[i] = d
			}
		}

		b, err := json.Marshal(Y)
		if err != nil {
			return err
		}
		if err := renameio.WriteFile(fmt.Sprintf("Y%04d.json", Y.Year), b, 0444); err != nil {
			return err
		}
	}

	var buf bytes.Buffer
	fmt.Fprintf(&buf,
		`// Generated with download.go on %s. DO NOT EDIT

package bankinap

var Years = map[uint16]YearDays{
`,
		time.Now().UTC().Format(time.RFC3339))

	for _, y := range slices.Sorted(maps.Keys(have)) {
		Y := have[y]
		fmt.Fprintf(&buf,
			"%d: YearDays{YearURL: YearURL{Year: %d, URL: %q},\nDays: []Day{\n", Y.Year, Y.Year, Y.URL)
		for _, d := range Y.Days {
			fmt.Fprintf(&buf, "{Date: Date{Year: %d, Month: %d, Day: %d}, Open: %t, Exchange: %t, BUX: %t},\n",
				d.Date.Year, d.Date.Month, d.Date.Day,
				d.Open, d.Exchange, d.BUX,
			)
		}
		buf.WriteString("}},\n")
	}

	buf.WriteString(`}
`)

	b, err := format.Source(buf.Bytes())
	if err != nil {
		os.Stderr.Write(buf.Bytes())
		return err
	}
	return renameio.WriteFile("downloaded.go", b, 0644)
}
