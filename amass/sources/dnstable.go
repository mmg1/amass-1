// Copyright 2017 Jeff Foley. All rights reserved.
// Use of this source code is governed by Apache 2 LICENSE that can be found in the LICENSE file.

package sources

import (
	"fmt"

	"github.com/OWASP/Amass/amass/utils"
)

type DNSTable struct {
	BaseDataSource
}

func NewDNSTable() DataSource {
	h := new(DNSTable)

	h.BaseDataSource = *NewBaseDataSource(SCRAPE, "DNSTable")
	return h
}

func (d *DNSTable) Query(domain, sub string) []string {
	var unique []string

	if domain != sub {
		return unique
	}

	url := d.getURL(domain)
	page, err := utils.GetWebPage(url, nil)
	if err != nil {
		d.log(fmt.Sprintf("%s: %v", url, err))
		return unique
	}

	re := utils.SubdomainRegex(domain)
	for _, sd := range re.FindAllString(page, -1) {
		if u := utils.NewUniqueElements(unique, sd); len(u) > 0 {
			unique = append(unique, u...)
		}
	}
	return unique
}

func (d *DNSTable) getURL(domain string) string {
	format := "https://dnstable.com/domain/%s"

	return fmt.Sprintf(format, domain)
}
