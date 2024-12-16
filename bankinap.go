// Copyright 2019, 2024 Tamás Gulácsi. All rights reserved.
//
// SPDX-License-Identifier: Apache-2.0

package bankinap

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"golang.org/x/net/html"

	"github.com/UNO-SOFT/zlog/v2"
	"github.com/xuri/excelize/v2"
)

func SearchXLSXURL(ctx context.Context, year int) (YearURL, error) {
	logger := zlog.SFromContext(ctx)
	var y YearURL
	base, err := url.Parse("https://www.mnb.hu/en/payments/settlement-systems/calendar")
	if err != nil {
		return y, err
	}
	_, rc, err := DownloadFile(ctx, base.String())
	if err != nil {
		return y, err
	}
	defer rc.Close()
	z := html.NewTokenizer(rc)
	candidates := make([]YearURL, 0, 512)
Loop:
	for {
		tt := z.Next()
		tagName, hasAttr := z.TagName()
		switch tt {
		case html.ErrorToken:
			err := z.Err()
			if errors.Is(err, io.EOF) {
				break Loop
			}
			return y, err

		case html.StartTagToken:
			if hasAttr && bytes.Equal(tagName, []byte("a")) {
				for {
					k, v, more := z.TagAttr()
					if bytes.Equal(k, []byte("href")) && bytes.IndexByte(v, ' ') < 0 {
						if _, rest, ok := bytes.Cut(v, []byte("/letoltes/")); ok {
							if rest, ok = bytes.CutSuffix(rest, []byte("-calendar.xlsx")); ok {
								if u, err := strconv.ParseUint(string(rest), 10, 16); err != nil {
									logger.Warn("parse", "rest", string(rest), "url", string(v), "error", err)
								} else {
									ref, err := url.Parse(string(v))
									if err != nil {
										logger.Error("parse", "url", v, "error", err)
									} else {
										candidates = append(candidates, YearURL{
											URL:  base.ResolveReference(ref).String(),
											Year: uint16(u),
										})
									}
								}
							}
						}
						break
					}
					if !more {
						break
					}
				}
			}
		}
	}
	rc.Close()

	// specified
	if Y := uint16(year); Y != 0 {
		for _, x := range candidates {
			if x.Year == Y {
				return x, nil
			}
		}
		return y, fmt.Errorf("year %d not found (have: %q)", Y, candidates)
	}

	// last (max)
	for _, x := range candidates {
		if y.Year < x.Year {
			y = x
		}
	}
	return y, nil
}

type YearURL struct {
	URL  string
	Year uint16
}

func Get(ctx context.Context, y YearURL) ([]Day, error) {
	_, rc, err := DownloadFile(ctx, y.URL)
	if err != nil {
		return nil, err
	}
	days, err := Parse(ctx, rc, int(y.Year))
	rc.Close()
	return days, err
}

func Parse(ctx context.Context, r io.Reader, year int) ([]Day, error) {
	logger := zlog.SFromContext(ctx)
	logger.Info("Parse")
	wb, err := excelize.OpenReader(r)
	if err != nil {
		return nil, err
	}
	sheet := wb.GetSheetName(0)
	rows, err := wb.Rows(sheet)
	if err != nil {
		return nil, err
	}
	logger.Info("got", "rows", rows)
	defer rows.Close()
	days := make([]Day, 0, 366)
	var row int
	for rows.Next() {
		row++
		cols, err := rows.Columns()
		if err != nil {
			return nil, err
		}
		if len(cols) <= 28 {
			continue
		}
		d := Date{Year: uint16(year)}
		switch strings.ToLower((cols[0] + "   ")[:3]) {
		case "jan":
			d.Month = time.January
		case "feb":
			d.Month = time.February
		case "mar":
			d.Month = time.March
		case "spr":
			d.Month = time.April
		case "may":
			d.Month = time.May
		case "jun":
			d.Month = time.June
		case "jul":
			d.Month = time.July
		case "aug":
			d.Month = time.August
		case "sep":
			d.Month = time.September
		case "oct":
			d.Month = time.October
		case "nov":
			d.Month = time.November
		case "dec":
			d.Month = time.December
		}
		if d.Month == 0 {
			continue
		}

		for i, c := range cols[1:] {
			d.Day = uint8(i + 1)
			cell, err := excelize.CoordinatesToCellName(i+2, row+0)
			if err != nil {
				return days, err
			}
			idx, err := wb.GetCellStyle(sheet, cell)
			if err != nil {
				return days, err
			}
			style, err := wb.GetStyle(idx)
			if err != nil {
				return days, err
			}
			day := Day{
				Date:     d,
				Open:     c == "O",
				Exchange: c == "O" && len(style.Fill.Color) != 0 && style.Fill.Color[0] == "FFFF00",
			}
			if false && (cell == "N14" || day.Exchange) {
				logger.Info(cell, "syle", style, "fill", len(style.Fill.Color), "open", c, "day", day)
			}
			days = append(days, day)
		}
	}

	return days, nil
}

type Day struct {
	Date     Date
	Open     bool `json:"Open,omitempty"`
	Exchange bool `json:"Exchange,omitempty"`
}

type Date struct {
	Year  uint16
	Month time.Month
	Day   uint8
}

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

func DownloadFile(ctx context.Context, dlURL string) (string, io.ReadCloser, error) {
	logger := zlog.SFromContext(ctx)
	logger.Info("DownloadFile", "url", dlURL)
	req, err := http.NewRequest("GET", dlURL, nil)
	if err != nil {
		return "", nil, fmt.Errorf("%s: %w", dlURL, err)
	}
	resp, err := http.DefaultClient.Do(req.WithContext(ctx))
	if err != nil {
		return "", nil, fmt.Errorf("%s: %w", dlURL, err)
	}
	cd := resp.Header.Get("Content-Disposition")
	var filename string
	if _, params, err := mime.ParseMediaType(cd); err == nil {
		filename = params["filename"]
	}
	return filename, resp.Body, nil
}
