package generate

import (
	"fmt"
	"strings"
	"time"

	"github.com/go-openapi/strfmt"
	models "github.com/grafana/grafana-openapi-client-go/models"
	"pgregory.net/rapid"
)

var (
	adjectives = []string{
		"Adorable", "Adventurous", "Agile", "Amazing", "Amicable", "Animated", "Artistic", "Astounding",
		"Beloved", "Blissful", "Bold", "Bouncy", "Brave", "Breezy", "Bright", "Brilliant",
		"Capable", "Charming", "Cheerful", "Clever", "Cosmic", "Courageous", "Creative", "Curious",
		"Daring", "Dazzling", "Dedicated", "Delightful", "Determined", "Dynamic", "Dapper", "Dancing",
		"Eager", "Eccentric", "Efficient", "Elegant", "Enchanted", "Energetic", "Enthusiastic", "Epic",
		"Fabulous", "Fantastic", "Fearless", "Festive", "Fiery", "Flying", "Friendly", "Frosty",
		"Gallant", "Gentle", "Gigantic", "Giggly", "Gleaming", "Glorious", "Graceful", "Grand",
		"Happy", "Harmonious", "Heroic", "Hilarious", "Honest", "Hopeful", "Humble", "Hyper",
		"Imaginative", "Incredible", "Inspired", "Intelligent", "Intrepid", "Inventive", "Jazzy", "Jolly",
		"Jovial", "Joyful", "Jubilant", "Jumping", "Keen", "Kinetic", "Kind", "Kooky",
		"Legendary", "Lively", "Loyal", "Lucky", "Luminous", "Lunar", "Majestic", "Marvelous",
		"Merry", "Mighty", "Mirthful", "Modest", "Mystical", "Natural", "Neat", "Nimble",
		"Noble", "Notable", "Optimistic", "Outstanding", "Peaceful", "Perfect", "Playful",
		"Pleasant", "Powerful", "Precious", "Proud", "Quirky", "Radiant", "Rapid", "Reliable",
		"Remarkable", "Resourceful", "Respected", "Roaring", "Robust", "Sailing", "Sensible", "Serene",
		"Shining", "Silly", "Sincere", "Skillful", "Smiling", "Snappy", "Soaring", "Sparkling",
		"Spectacular", "Speedy", "Spirited", "Splendid", "Stellar", "Strong", "Stunning", "Sunny",
		"Supreme", "Swift", "Talented", "Tender", "Thriving", "Tidy", "Tremendous", "Trusty",
		"Ultimate", "Unbeatable", "Unique", "United", "Upbeat", "Valiant", "Vibrant", "Victorious",
		"Vigorous", "Vivacious", "Vivid", "Wacky", "Warm", "Whimsical", "Wise", "Witty",
		"Wonderful", "Worthy", "Xenial", "Youthful", "Yummy", "Zany", "Zealous", "Zesty", "Zippy",
	}

	animals = []string{
		"Albatross", "Alligator", "Alpaca", "Angelfish", "Antelope",
		"Badger", "Barracuda", "Bat", "Bear", "Beaver", "Bee", "Bison", "Buffalo", "Butterfly",
		"Camel", "Capybara", "Caribou", "Catfish", "Chameleon", "Cheetah", "Chickadee",
		"Chipmunk", "Cockatoo", "Condor", "Cougar", "Coyote", "Crane",
		"Crocodile", "Crow", "Deer", "Dingo", "Dolphin", "Donkey", "Dove", "Dragonfly",
		"Duck", "Eagle", "Eel", "Elephant", "Elk", "Emu",
		"Falcon", "Ferret", "Finch", "Firefly", "Flamingo", "Fox", "Frog",
		"Gazelle", "Gecko", "Gerbil", "Gibbon", "Giraffe", "Gopher", "Gorilla", "Grasshopper", "Grizzly",
		"Hamster", "Hare", "Hawk", "Hedgehog", "Heron", "Hippo", "Hummingbird",
		"Iguana", "Impala", "Jackal", "Jaguar", "Jellyfish",
		"Kangaroo", "Kingfisher", "Kiwi", "Koala", "Kookaburra",
		"Ladybug", "Lemur", "Leopard", "Lion", "Lionfish", "Llama", "Lobster",
		"Lynx", "Macaw", "Magpie", "Manatee", "Meerkat",
		"Mongoose", "Moose", "Moth", "Narwhal", "Nightingale",
		"Octopus", "Opossum", "Orangutan", "Ostrich", "Otter", "Owl", "Ox",
		"Panda", "Panther", "Parrot", "Peacock", "Pelican", "Penguin", "Phoenix", "Porcupine",
		"Puffin", "Quail", "Quokka", "Rabbit", "Raccoon", "Raven", "Reindeer", "Rhino",
		"Roadrunner", "Salamander", "Salmon", "Seagull", "Seahorse", "Seal", "Shark",
		"Sheep", "Sloth", "Snail", "Sparrow", "Squirrel",
		"Starfish", "Stingray", "Stork", "Swallow", "Swan", "Swordfish",
		"Tiger", "Toad", "Toucan", "Trout", "Tuna", "Turkey", "Turtle", "Unicorn",
		"Vulture", "Wallaby", "Walrus", "Whale", "Wildcat", "Wolf",
		"Woodpecker", "Wombat", "Yak", "Zebra",
	}
)

var queries = []string{
	"vector(0)",
	"vector(1)",
	"sum by (name) (group by (id, name) (grafanacloud_instance_info))",
	"sum by (plan) (group by (id, plan) (grafanacloud_grafana_instance_info))",
	"sum by (id, state) (grafanacloud_grafana_instance_active_user_count)",
	"grafanacloud_instance_rule_evaluation_failures_total:rate5m > 0",
	"grafanacloud_instance_ruler_notifications_errors_total:rate5m > 0",
	"grafanacloud_org_total_overage > 0",
	"grafanacloud_org_spend_commit_balance_total == 0 or grafanacloud_org_spend_commit_balance_total < grafanacloud_org_spend_commit_credit_total * 0.1",
	"sum by (id, state) (grafanacloud_grafana_instance_alerting_alerts)",
	"sum by (id, state) (grafanacloud_grafana_instance_alerting_alerts) > 10",
	"sum by (id, state) (grafanacloud_grafana_instance_alerting_alerts) > 25",
	"sum by (id, state) (grafanacloud_grafana_instance_alerting_alerts) > 50",
	"sum by (id, state) (grafanacloud_grafana_instance_alerting_alerts) > 100",
	"sum by (id, state) (grafanacloud_grafana_instance_alerting_alerts) > 500",
	"sum by (id, state) (grafanacloud_grafana_instance_alerting_alerts) > 1000",
	"sum by (id, state) (grafanacloud_grafana_instance_alerting_alerts) > 2500",
	"sum by (id, state) (grafanacloud_grafana_instance_alerting_alerts) > 5000",
	"sum by (id, state) (grafanacloud_grafana_instance_alerting_alerts) > 10000",
	"sum by (id, state) (grafanacloud_grafana_instance_alerting_alerts) > 50000",
	"sum by (id, state) (grafanacloud_grafana_instance_alerting_alerts) > 100000",
}

// Helpers
func buildQuery(t *rapid.T, dsUID, refID string) *models.AlertQuery {
	// __expr__ math or a basic prom query
	model := map[string]any{
		"refId":      refID,
		"type":       "math",
		"expression": "1 == 1",
		"datasource": map[string]any{"type": "__expr__", "uid": "__expr__"},
	}
	if dsUID != "__expr__" {
		model = map[string]any{
			"refId":      refID,
			"type":       "query",
			"datasource": map[string]any{"uid": dsUID},
			"expr":       rapid.SampledFrom(queries).Draw(t, "query"),
			"instant":    true,
			"range":      false,
		}
	}
	return &models.AlertQuery{
		DatasourceUID:     dsUID,
		Model:             model,
		QueryType:         "",
		RefID:             refID,
		RelativeTimeRange: &models.RelativeTimeRange{From: models.Duration(600), To: models.Duration(0)},
	}
}

func genTitle() *rapid.Generator[string] {
	return rapid.Custom(func(t *rapid.T) string {
		adj := rapid.SampledFrom(adjectives).Draw(t, "adjective")
		animal := rapid.SampledFrom(animals).Draw(t, "animal")
		uid := rapid.StringMatching(`[A-Za-z0-9]{5,10}`).Draw(t, "uid_suffix")
		return fmt.Sprintf("%s %s [%s]", adj, animal, uid)
	})
}

func genSummary() *rapid.Generator[string] {
	return rapid.StringMatching(`[A-Za-z0-9 .,!?-]{10,60}`)
}

func genDurationStr() *rapid.Generator[string] {
	return rapid.SampledFrom([]string{"0s", "30s", "1m", "5m", "10m"})
}

func genLabels() *rapid.Generator[map[string]string] {
	keys := []string{"team", "service", "env", "region"}
	val := rapid.StringMatching(`[a-z][a-z0-9\-]{2,10}`)
	keyFn := func(s string) string {
		if len(s) == 0 {
			return keys[0]
		}
		var h uint32
		for i := 0; i < len(s); i++ {
			h = h*16777619 ^ uint32(s[i])
		}
		return keys[int(h)%len(keys)]
	}
	return rapid.MapOfValues(val, keyFn)
}

func genMetricName() *rapid.Generator[string] {
	return rapid.Custom(func(t *rapid.T) string {
		prefix := rapid.SampledFrom([]string{"grafana", "app", "service", "custom"}).Draw(t, "prefix")
		parts := rapid.SliceOfN(rapid.StringMatching(`[a-z][a-z0-9_]{2,8}`), 1, 3).Draw(t, "parts")
		return prefix + "_" + strings.Join(parts, "_")
	})
}

func RandomUID() *rapid.Generator[string] {
	return rapid.StringMatching(`[A-Za-z0-9\-_]{8,16}`)
}

// genAdditionalAnnotations returns a random small map of annotation keys -> values (excluding summary)
func genAdditionalAnnotations() *rapid.Generator[map[string]string] {
	keys := []string{"runbook_url", "dashboard", "description", "priority", "owner", "ticket"}
	return rapid.Custom(func(t *rapid.T) map[string]string {
		n := rapid.IntRange(0, 4).Draw(t, "ann_n")
		m := make(map[string]string)
		for i := 0; i < n; i++ {
			k := rapid.SampledFrom(keys).Draw(t, "ann_key")
			if _, exists := m[k]; exists {
				continue
			}
			var v string
			switch k {
			case "runbook_url", "dashboard":
				v = genURL().Draw(t, "ann_url")
			case "priority":
				v = rapid.SampledFrom([]string{"P1", "P2", "P3", "P4"}).Draw(t, "ann_priority")
			case "owner":
				v = rapid.StringMatching(`[a-z][a-z0-9_\-]{2,12}`).Draw(t, "ann_owner")
			case "ticket":
				v = rapid.StringMatching(`[A-Z]{2,4}-[0-9]{2,5}`).Draw(t, "ann_ticket")
			default:
				v = genSummary().Draw(t, "ann_desc")
			}
			m[k] = v
		}
		return m
	})
}

func genURL() *rapid.Generator[string] {
	return rapid.Custom(func(t *rapid.T) string {
		host := rapid.SampledFrom([]string{"example.com", "grafana.net", "runbooks.local"}).Draw(t, "url_host")
		segs := rapid.SliceOfN(rapid.StringMatching(`[a-z0-9\-]{3,10}`), 1, 3).Draw(t, "url_segs")
		return "https://" + host + "/" + strings.Join(segs, "/")
	})
}

func strPtr[T ~string](s T) *T { return &s }

// genExecErrState returns a valid exec error state value
func genExecErrState() *rapid.Generator[string] {
	return rapid.SampledFrom([]string{"OK", "Alerting", "Error"})
}

// genNoDataState returns a valid no data state value
func genNoDataState() *rapid.Generator[string] {
	return rapid.SampledFrom([]string{"Alerting", "NoData", "OK"})
}

func mustParseDuration(s string) strfmt.Duration {
	if s == "" {
		return strfmt.Duration(0)
	}
	d, err := time.ParseDuration(s)
	if err != nil {
		return strfmt.Duration(0)
	}
	return strfmt.Duration(d)
}
