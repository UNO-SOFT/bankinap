// Copyright 2019, 2026 Tamás Gulácsi. All rights reserved.
//
// SPDX-License-Identifier: AGPL-3.0-or-later

package bankinap

import (
	_ "embed"
	"encoding/json"
	// "fmt"
	"time"

	"github.com/UNO-SOFT/bankinap/rle"
)

//go:embed downloaded.json
var embedded []byte

var Years map[uint16]YearDays

type (
	PQDate int32 // days since epoch

	PQYear struct {
		YearURL
		Open, Exchange, BUX []byte `json:",omitzero"` // RLE encoded bytes
	}
)

func init() {
	if len(embedded) == 0 {
		return
	}
	yy := make([]PQYear, 0, 8)
	if err := func() error {
		if err := json.Unmarshal(embedded, &yy); err != nil {
			return err
		}
		if Years == nil {
			Years = make(map[uint16]YearDays, len(yy))
		}
		for _, Y := range yy {
			oo, err := rle.Decode(Y.Open)
			if err != nil {
				return err
			}
			ee, err := rle.Decode(Y.Exchange)
			if err != nil {
				return err
			}
			bb, err := rle.Decode(Y.BUX)
			if err != nil {
				return err
			}
			days := make([]Day, len(oo))
			dt := NewPQDate(int(Y.Year), time.January, 1)
			for i := range oo {
				days[i] = Day{Date: dt.Date(), Open: oo[i], Exchange: ee[i], BUX: bb[i]}
				dt++

				// if d := days[i]; !d.Open {
				// 	fmt.Println(d)
				// }
			}
			Years[Y.Year] = YearDays{
				YearURL: Y.YearURL,
				Days:    days,
			}
		}
		return nil
	}(); err != nil {
		panic(err)
	}
}

func NewPQDate(year int, month time.Month, day int) PQDate {
	return PQDate(time.Date(year, month, day,
		0, 0, 0, 0, time.UTC).Unix() / 24 / 3600)
}

func (d PQDate) Time() time.Time {
	return time.Unix(int64(d)*24*3600, 0)

}
func (d PQDate) Date() Date {
	t := d.Time()
	return Date{Year: uint16(t.Year()), Month: t.Month(), Day: uint8(t.Day())}
}
