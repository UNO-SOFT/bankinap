// Copyright 2019, 2026 Tamás Gulácsi. All rights reserved.
//
// SPDX-License-Identifier: AGPL-3.0-or-later

package main

import (
	"context"
	"fmt"
	"log"
	"maps"
	"os"
	"os/signal"
	"slices"

	"github.com/UNO-SOFT/bankinap"
	"github.com/UNO-SOFT/bankinap/download"

	"github.com/parquet-go/parquet-go"
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
	have := bankinap.Years
	if have == nil {
		have = make(map[uint16]bankinap.YearDays)
	}
	for _, y := range yy {
		if days, ok := have[y.Year]; ok && len(days.Days) >= 365 {
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
	}

	rows := make([]bankinap.PQDay, 0, len(have)*(365*4+1)/4)
	opts := append(make([]parquet.WriterOption, 0, 1+len(have)),
		parquet.Compression(&parquet.Lz4Raw))
	for _, y := range slices.Sorted(maps.Keys(have)) {
		Y := have[y]
		opts = append(opts, parquet.KeyValueMetadata(fmt.Sprintf("%d.url", Y.Year), Y.URL))
		for _, d := range Y.Days {
			rows = append(rows, bankinap.PQDay{
				Date: bankinap.NewPQDate(int(d.Date.Year), d.Date.Month, int(d.Date.Day)),
				Open: d.Open, Exchange: d.Exchange, BUX: d.BUX,
			})
		}
	}

	return parquet.WriteFile("downloaded.parquet", rows, opts...)
}
