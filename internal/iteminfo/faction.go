package iteminfo

import "strings"

// taskFaction is the faction a task is for, from the catalog's factionName:
// "BEAR" or "USEC", "" for both ("Any").
func taskFaction(name string) string {
	switch strings.ToUpper(strings.TrimSpace(name)) {
	case "BEAR":
		return "BEAR"
	case "USEC":
		return "USEC"
	}
	return ""
}

// ForFaction leaves out the tasks of the other faction when the player's
// faction ("USEC", "BEAR") is known: some tasks come as a BEAR and a USEC
// twin with the same name and items (Textile, Drip-Out), and the twin of the
// other faction would stay needed for good. With faction "" it keeps all.
func ForFaction(info Info, faction string) Info {
	faction = taskFaction(faction)
	if faction == "" {
		return info
	}
	tasks := make([]TaskNeed, 0, len(info.Tasks))
	for _, task := range info.Tasks {
		if task.Faction == "" || task.Faction == faction {
			tasks = append(tasks, task)
		}
	}
	info.Tasks = tasks
	return info
}
