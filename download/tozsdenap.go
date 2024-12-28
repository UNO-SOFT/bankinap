// Copyright 2024 Tamás Gulácsi. All rights reserved.
//
// SPDX-License-Identifier: Apache-2.0

package download

import (
	"context"
	"regexp"
	"strconv"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/UNO-SOFT/zlog/v2"

	"github.com/UNO-SOFT/bankinap"
)

const DefaultBETURL = `https://www.bet.hu/aktualis`

func SearchBUX(ctx context.Context, URL string) ([]bankinap.BUXHoliday, error) {
	logger := zlog.SFromContext(ctx)
	_ = logger
	if URL == "" {
		URL = DefaultBETURL
	}
	_, rc, err := DownloadFile(ctx, URL)
	if err != nil {
		return nil, err
	}
	defer rc.Close()
	doc, err := goquery.NewDocumentFromReader(rc)
	if err != nil {
		return nil, err
	}

	rYear := regexp.MustCompile(" 2[0-9]{3}-")
	rMD := regexp.MustCompile(`(január|február|március|április|május|június|július|augusztus|szeptember|október|november|december) ([1-9]|[12][0-9]|3[01])([^0-9]|$)`)

	holidays := make([]bankinap.BUXHoliday, 0, 16)
	doc.Find("article tbody").Each(func(i int, s *goquery.Selection) {
		var year int64
		s.Find("tr").Each(func(_ int, s *goquery.Selection) {
			fields := make([]string, 0, 2)
			s.Find("td").Each(func(_ int, s *goquery.Selection) {
				fields = append(fields, s.Text())
			})
			if year == 0 {
				s := rYear.FindString(fields[0])[1:]
				var y int64
				y, err = strconv.ParseInt(s[:len(s)-1], 10, 16)
				if err == nil {
					year = y
				}
			}
			if len(fields) == 2 {
				// fmt.Println(i, year, fields[0])
				ii := rMD.FindStringSubmatch(fields[0])
				var m time.Month
				switch ii[1][:3] {
				case "jan":
					m = time.January
				case "feb":
					m = time.February
				case "már":
					m = time.March
				case "máj":
					m = time.May
				case "jún":
					m = time.June
				case "júl":
					m = time.July
				case "aug":
					m = time.August
				case "sze":
					m = time.September
				case "okt":
					m = time.October
				case "nov":
					m = time.November
				case "dec":
					m = time.December
				}
				var d uint64
				if d, err = strconv.ParseUint(ii[2], 10, 8); err == nil {
					holidays = append(holidays, bankinap.BUXHoliday{
						Date: bankinap.Date{
							Year: uint16(year), Month: m, Day: uint8(d)},
						Comment: fields[1],
					})
				}
			}
		})
	})

	// .article-body > table:nth-child(3) > tbody:nth-child(1) > tr:nth-child(9) > td:nth-child(1)
	// html.mouse-intent body div.wrapper main.subpage-root.page-container.page-row.page-editor-columns section.center.column div.inner div div.portlet.ContentViewPortlet div div.content-view-content.bet-articles article div.article-body table tbody tr td

	return holidays, err
}
