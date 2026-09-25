// Command catalogcheck downloads the tarkov.dev catalog for game modes and
// prints its counts and the tasks it yields, to check the catalog end to end.
//
//	go run ./cmd/catalogcheck [regular|pve|pvp-season|auto]...
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/local/mayak/internal/catalog"
	"github.com/local/mayak/internal/questapi"
)

func main() {
	modes := os.Args[1:]
	if len(modes) == 0 {
		modes = []string{"regular", "pve", "pvp-season", "auto"}
	}
	data := catalog.New()
	client := questapi.NewWithSource(data)
	for _, mode := range modes {
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		if catalog.ValidMode(mode) {
			snapshot, err := data.Refresh(ctx, mode, true)
			if err != nil {
				cancel()
				log.Fatal(err)
			}
			fmt.Printf("mode=%s items=%d maps=%d traders=%d tasks=%d hideout=%d playerLevels=%d scavCooldown=%ds\n", mode, snapshot.Items, snapshot.Maps, snapshot.Traders, snapshot.Tasks, snapshot.HideoutStations, snapshot.PlayerLevels, snapshot.ScavCooldownSeconds)
		}
		quests, err := client.QuestsForMode(ctx, mode)
		cancel()
		if err != nil {
			log.Fatal(err)
		}
		fmt.Printf("mode=%s loaded=%d first=%q\n", mode, len(quests), quests[0].Name)
	}
}
