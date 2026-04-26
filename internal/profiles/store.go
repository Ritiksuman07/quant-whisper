package profiles

import (
    "encoding/json"
    "errors"
    "os"
    "path/filepath"
    "strings"
)

func configPath() (string, error) {
    dir, err := os.UserConfigDir()
    if err != nil {
        return "", err
    }
    return filepath.Join(dir, "portman", "profiles.json"), nil
}

func Load() ([]Profile, error) {
    path, err := configPath()
    if err != nil {
        return nil, err
    }
    data, err := os.ReadFile(path)
    if err != nil {
        if errors.Is(err, os.ErrNotExist) {
            return []Profile{}, nil
        }
        return nil, err
    }

    var profiles []Profile
    if err := json.Unmarshal(data, &profiles); err != nil {
        return nil, err
    }
    return profiles, nil
}

func Save(profiles []Profile) error {
    path, err := configPath()
    if err != nil {
        return err
    }
    if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
        return err
    }

    data, err := json.MarshalIndent(profiles, "", "  ")
    if err != nil {
        return err
    }
    return os.WriteFile(path, data, 0o600)
}

func ExpandHome(path string) string {
    if path == "" || path[0] != '~' {
        return path
    }
    home, err := os.UserHomeDir()
    if err != nil {
        return path
    }
    if path == "~" {
        return home
    }
    if strings.HasPrefix(path, "~/") {
        return filepath.Join(home, path[2:])
    }
    return path
}
