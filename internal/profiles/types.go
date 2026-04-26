package profiles

import "time"

type Profile struct {
    Name           string           `json:"name"`
    Description    string           `json:"description"`
    Commands       []ProfileCommand `json:"commands"`
    CreatedAt      time.Time        `json:"created_at"`
    LastLaunchedAt *time.Time       `json:"last_launched_at"`
}

type ProfileCommand struct {
    Label string   `json:"label"`
    Cmd   string   `json:"cmd"`
    Port  int      `json:"port"`
    Dir   string   `json:"dir"`
    Env   []string `json:"env"`
}
