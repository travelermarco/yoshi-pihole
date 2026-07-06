package gravity

import (
	"context"
	"log"

	"yoshi-pihole/internal/db"
)

// Builder refreshes the gravity domain corpus from all enabled adlist
// subscriptions, mirroring Pi-hole's "pihole -g" / gravity.sh.
type Builder struct {
	Store   *db.GravityStore
	Fetcher *Fetcher
}

func NewBuilder(store *db.GravityStore) *Builder {
	return &Builder{Store: store, Fetcher: NewFetcher()}
}

// Run fetches every enabled adlist (using ETag caching to skip unchanged
// sources), parses it, and replaces its compiled domain set in gravity.db.
// Errors on individual adlists are logged and skipped rather than aborting
// the whole run, so one broken subscription doesn't block the rest.
func (b *Builder) Run(ctx context.Context) error {
	adlists, err := b.Store.ListAdlists()
	if err != nil {
		return err
	}

	for _, a := range adlists {
		if !a.Enabled {
			continue
		}

		body, newEtag, notModified, err := b.Fetcher.Fetch(ctx, a.Address, a.ETag)
		if err != nil {
			log.Printf("gravity: fetching %s: %v", a.Address, err)
			_ = b.Store.UpdateAdlistMeta(a.ID, a.Number, a.InvalidDomains, "error", a.ETag)
			continue
		}
		if notModified {
			continue
		}

		result := DetectAndParse(body)
		if err := b.Store.ReplaceGravityDomains(a.ID, result.Domains); err != nil {
			log.Printf("gravity: storing domains for %s: %v", a.Address, err)
			continue
		}
		if err := b.Store.UpdateAdlistMeta(a.ID, len(result.Domains), result.InvalidDomains, "ok", newEtag); err != nil {
			log.Printf("gravity: updating metadata for %s: %v", a.Address, err)
		}
	}

	return nil
}

// EnsureDefaultAdlists seeds the configured default blocklists on first run
// (idempotent: AddAdlist relies on the adlist.address UNIQUE constraint).
func EnsureDefaultAdlists(store *db.GravityStore, urls []string) error {
	existing, err := store.ListAdlists()
	if err != nil {
		return err
	}
	have := map[string]bool{}
	for _, a := range existing {
		have[a.Address] = true
	}

	for _, url := range urls {
		if have[url] {
			continue
		}
		if _, err := store.AddAdlist(url, db.AdlistBlock, "default"); err != nil {
			return err
		}
	}
	return nil
}
