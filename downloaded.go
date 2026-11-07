// Copyright 2019, 2024 Tamás Gulácsi. All rights reserved.
//
// SPDX-License-Identifier: AGPL-3.0-or-later

package bankinap

import (
	"bytes"
	_ "embed"
	"fmt"
	"time"

	"github.com/parquet-go/parquet-go"
)

//go:embed downloaded.parquet
var pqBytes []byte

var Years map[uint16]YearDays

type (
	PQDate int32 // days since epoch

	PQDay struct {
		Date                PQDate `parquet:"date,delta"`
		Open, Exchange, BUX bool
	}
)

func init() {
	if len(pqBytes) == 0 {
		return
	}
	years := make([]YearURL, 0, 8)
	f, err := parquet.OpenFile(bytes.NewReader(pqBytes), int64(len(pqBytes)))
	if err != nil {
		panic(err)
	}
	rows, err := parquet.Read[PQDay](bytes.NewReader(pqBytes), int64(len(pqBytes)))

	Years = make(map[uint16]YearDays, len(years))
	m := make(map[uint16]*[]Day, len(years))
	for _, d := range rows {
		t := d.Date.Time()
		y := uint16(t.Year())
		if _, ok := Years[y]; !ok {
			url, _ := f.Lookup(fmt.Sprintf("%d.url", y))
			Years[y] = YearDays{
				YearURL: YearURL{Year: y, URL: url},
			}
			days := make([]Day, 0, 366)
			m[y] = &days
		}
		*m[y] = append(*m[y], Day{Date: d.Date.Date()})
	}
	for y, Y := range Years {
		Y.Days = *m[y]
		Years[y] = Y
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
