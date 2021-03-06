// Copyright 2018 Frédéric Guillot. All rights reserved.
// Use of this source code is governed by the Apache 2.0
// license that can be found in the LICENSE file.

package processor

import (
	"regexp"
	"time"

	"miniflux.app/config"
	"miniflux.app/logger"
	"miniflux.app/metric"
	"miniflux.app/model"
	"miniflux.app/reader/rewrite"
	"miniflux.app/reader/sanitizer"
	"miniflux.app/reader/scraper"
	"miniflux.app/storage"
)

// ProcessFeedEntries downloads original web page for entries and apply filters.
func ProcessFeedEntries(store *storage.Storage, feed *model.Feed) {

	filterFeedEntries(feed)

	for _, entry := range feed.Entries {
		logger.Debug("[Feed #%d] Processing entry %s", feed.ID, entry.URL)
		if feed.Crawler {
			if !store.EntryURLExists(feed.ID, entry.URL) {
				startTime := time.Now()
				content, scraperErr := scraper.Fetch(entry.URL, feed.ScraperRules, feed.UserAgent)

				if config.Opts.HasMetricsCollector() {
					status := "success"
					if scraperErr != nil {
						status = "error"
					}
					metric.ScraperRequestDuration.WithLabelValues(status).Observe(time.Since(startTime).Seconds())
				}

				if scraperErr != nil {
					logger.Error(`[Filter] Unable to crawl this entry: %q => %v`, entry.URL, scraperErr)
				} else if content != "" {
					// We replace the entry content only if the scraper doesn't return any error.
					entry.Content = content
				}
			}
		}

		entry.Content = rewrite.Rewriter(entry.URL, entry.Content, feed.RewriteRules)

		// The sanitizer should always run at the end of the process to make sure unsafe HTML is filtered.
		entry.Content = sanitizer.Sanitize(entry.URL, entry.Content)
	}
}

/*
Filters feed entries based on regex rules
First we filter based on our keep list, then we remove those entries that match the block list
*/
func filterFeedEntries(feed *model.Feed) {
	var filteredEntries []*model.Entry

	if len(feed.KeeplistRules) > 0 {
		for _, entry := range feed.Entries {
			match, _ := regexp.MatchString(feed.KeeplistRules, entry.Title)
			if match == true {
				filteredEntries = append(filteredEntries, entry)
			}
		}
	} else {
		filteredEntries = feed.Entries
	}
	if len(feed.BlocklistRules) > 0 {
		k := 0
		for _, entry := range filteredEntries {
			match, _ := regexp.MatchString(feed.BlocklistRules, entry.Title)
			if match != true {
				filteredEntries[k] = entry
				k++
			}
		}
		filteredEntries = filteredEntries[:k]
	}
	feed.Entries = filteredEntries
}

// ProcessEntryWebPage downloads the entry web page and apply rewrite rules.
func ProcessEntryWebPage(entry *model.Entry) error {
	startTime := time.Now()
	content, scraperErr := scraper.Fetch(entry.URL, entry.Feed.ScraperRules, entry.Feed.UserAgent)
	if config.Opts.HasMetricsCollector() {
		status := "success"
		if scraperErr != nil {
			status = "error"
		}
		metric.ScraperRequestDuration.WithLabelValues(status).Observe(time.Since(startTime).Seconds())
	}

	if scraperErr != nil {
		return scraperErr
	}

	content = rewrite.Rewriter(entry.URL, content, entry.Feed.RewriteRules)
	content = sanitizer.Sanitize(entry.URL, content)

	if content != "" {
		entry.Content = content
	}

	return nil
}
