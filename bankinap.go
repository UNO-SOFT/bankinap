// Copyright 2019, 2024 Tamás Gulácsi. All rights reserved.
//
// SPDX-License-Identifier: Apache-2.0

package bankinap

import (
	"fmt"
	"time"
)

//go:generate go run gen.go

type YearURL struct {
	URL  string
	Year uint16
}

type YearDays struct {
	YearURL
	Days []Day
}

type Day struct {
	Date     Date
	Open     bool `json:"Open,omitempty"`
	Exchange bool `json:"Exchange,omitempty"`
}

func (d Day) Compare(other Day) int { return d.Date.Compare(other.Date) }

type Date struct {
	Year  uint16
	Month time.Month
	Day   uint8
}

func (d Date) Compare(other Date) int { return d.Time().Compare(other.Time()) }

func (d Date) Time() time.Time {
	return time.Date(int(d.Year), d.Month, int(d.Day), 0, 0, 0, 0, time.Local)
}

func (d Date) String() string {
	return fmt.Sprintf("%04d-%02d-%02d", d.Year, d.Month, d.Day)
}

func (d Date) MarshalText() ([]byte, error) { return []byte(d.String()), nil }
func (d Date) MarshalJSON() ([]byte, error) {
	return append(append(append(
		make([]byte, 0, 1+10+1), '"'), d.String()...), '"'), nil
}
func (d *Date) UnmarshalJSON(p []byte) error {
	_, err := fmt.Sscanf(string(p), `"%04d-%02d-%02d"`, &d.Year, &d.Month, &d.Day)
	return err
}
