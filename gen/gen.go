// Copyright 2019, 2026 Tamás Gulácsi. All rights reserved.
//
// SPDX-License-Identifier: AGPL-3.0-or-later

package main

import (
	"context"
	"encoding/json"
	"log"
	"os"
	"os/signal"

	"github.com/UNO-SOFT/bankinap"
	"github.com/UNO-SOFT/bankinap/download"
	"github.com/UNO-SOFT/bankinap/rle"

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

	pqyy := make([]bankinap.PQYear, 0, len(have))
	oo := make([]bool, 0, 366)
	ee := make([]bool, 0, 366)
	bb := make([]bool, 0, 366)
	for _, Y := range have {
		oo, ee, bb = oo[:0], ee[:0], bb[:0]
		for _, d := range Y.Days {
			oo = append(oo, d.Open)
			ee = append(ee, d.Exchange)
			bb = append(bb, d.BUX)
		}
		pqyy = append(pqyy, bankinap.PQYear{
			YearURL:  Y.YearURL,
			Open:     rle.Encode(oo),
			Exchange: rle.Encode(ee),
			BUX:      rle.Encode(bb),
		})
	}
	b, err := json.Marshal(pqyy)
	if err != nil {
		return err
	}
	return renameio.WriteFile("downloaded.json", b, 0444)
}
