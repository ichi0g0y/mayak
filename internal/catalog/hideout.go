package catalog

import (
	"context"
	"encoding/json"
	"sort"
)

type HideoutStation struct {
	ID             string         `json:"id"`
	Name           string         `json:"name"`
	NormalizedName string         `json:"normalizedName"`
	AreaType       int            `json:"areaType"`
	Levels         []HideoutLevel `json:"levels"`
}
type HideoutLevel struct {
	ID                       string                      `json:"id"`
	Level                    int                         `json:"level"`
	ConstructionTime         int                         `json:"constructionTime"`
	ItemRequirements         []HideoutItemRequirement    `json:"itemRequirements"`
	StationLevelRequirements []HideoutStationRequirement `json:"stationLevelRequirements"`
	TraderRequirements       []HideoutTraderRequirement  `json:"traderRequirements"`
	SkillRequirements        []HideoutSkillRequirement   `json:"skillRequirements"`
}
type HideoutItemRequirement struct {
	Item       string                `json:"item"`
	Name       string                `json:"name"`
	Count      float64               `json:"count"`
	Attributes HideoutItemAttributes `json:"attributes"`
}
type HideoutItemAttributes struct {
	FoundInRaid bool `json:"foundInRaid"`
}

type HideoutStationRequirement struct {
	Station string `json:"station"`
	Name    string `json:"name"`
	Level   int    `json:"level"`
}
type HideoutTraderRequirement struct {
	Trader string `json:"trader"`
	Name   string `json:"name"`
	Level  int    `json:"level"`
}
type HideoutSkillRequirement struct {
	Skill string `json:"skill"`
	Level int    `json:"level"`
}

func (c *Client) Hideout(ctx context.Context, mode string) ([]HideoutStation, error) {
	snapshot, err := c.Refresh(ctx, mode, false)
	if snapshot == nil {
		return nil, err
	}
	decode := func(resource string, target any) error { return json.Unmarshal(snapshot.Resources[resource], target) }
	var stations struct {
		Data map[string]HideoutStation `json:"data"`
	}
	if err := decode("hideout", &stations); err != nil {
		return nil, err
	}
	var names struct {
		Data map[string]string `json:"data"`
	}
	if err := decode("hideout_en", &names); err != nil {
		return nil, err
	}
	var items struct {
		Data struct {
			Items map[string]struct {
				Name string `json:"name"`
			} `json:"items"`
		} `json:"data"`
	}
	_ = decode("items", &items)
	var itemNames struct {
		Data map[string]string `json:"data"`
	}
	_ = decode("items_en", &itemNames)
	var traders struct {
		Data map[string]struct {
			Name string `json:"name"`
		} `json:"data"`
	}
	_ = decode("traders", &traders)
	var traderNames struct {
		Data map[string]string `json:"data"`
	}
	_ = decode("traders_en", &traderNames)
	localized := func(key string, dict map[string]string) string {
		if v := dict[key]; v != "" {
			return v
		}
		return key
	}
	var result []HideoutStation
	for _, station := range stations.Data {
		station.Name = localized(station.Name, names.Data)
		sort.Slice(station.Levels, func(i, j int) bool { return station.Levels[i].Level < station.Levels[j].Level })
		for i := range station.Levels {
			level := &station.Levels[i]
			for j := range level.ItemRequirements {
				r := &level.ItemRequirements[j]
				r.Name = localized(items.Data.Items[r.Item].Name, itemNames.Data)
			}
			for j := range level.StationLevelRequirements {
				r := &level.StationLevelRequirements[j]
				r.Name = localized(stations.Data[r.Station].Name, names.Data)
			}
			for j := range level.TraderRequirements {
				r := &level.TraderRequirements[j]
				r.Name = localized(traders.Data[r.Trader].Name, traderNames.Data)
			}
		}
		result = append(result, station)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Name < result[j].Name })
	return result, nil
}
