package game

import (
	"archive/zip"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"

	"github.com/OoTMM/multiplayer/client/internal/config"
)

type InfoMode int

const (
	InfoModeSingle InfoMode = 0
	InfoModeCoop   InfoMode = 1
	InfoModeMulti  InfoMode = 2
)

type Info struct {
	SessionID     [16]byte
	SessionSecret [8]byte
	WorldID       uint8
	Mode          InfoMode
}

func ExtractGameInfo(conf *config.Config) (*Info, error) {
	reader, err := zip.OpenReader(conf.GamePatchPath)
	if err != nil {
		return nil, err
	}
	defer reader.Close()

	metaFile, err := reader.Open("meta.json")
	if err != nil {
		return nil, err
	}
	defer metaFile.Close()

	type rawMetaFile struct {
		Meta struct {
			SessionID     string `json:"sessionId"`
			SessionSecret string `json:"sessionSecret"`
			WorldID       uint8  `json:"worldId"`
			Mode          string `json:"mode"`
		} `json:"meta"`
	}

	metaData, err := io.ReadAll(metaFile)
	if err != nil {
		return nil, err
	}

	var rawMeta rawMetaFile
	err = json.Unmarshal(metaData, &rawMeta)
	if err != nil {
		return nil, err
	}

	sessionID, err := hex.DecodeString(rawMeta.Meta.SessionID)
	if err != nil {
		return nil, err
	}
	if len(sessionID) != 16 {
		return nil, fmt.Errorf("invalid session ID length: expected 16 bytes, got %d", len(sessionID))
	}

	sessionSecret, err := hex.DecodeString(rawMeta.Meta.SessionSecret)
	if err != nil {
		return nil, err
	}
	if len(sessionSecret) != 8 {
		return nil, fmt.Errorf("invalid session secret length: expected 8 bytes, got %d", len(sessionSecret))
	}

	var info Info
	copy(info.SessionID[:], sessionID)
	copy(info.SessionSecret[:], sessionSecret)
	info.WorldID = rawMeta.Meta.WorldID

	switch rawMeta.Meta.Mode {
	case "single":
		info.Mode = InfoModeSingle
	case "coop":
		info.Mode = InfoModeCoop
	case "multi":
		info.Mode = InfoModeMulti
	default:
		return nil, fmt.Errorf("unknown mode: %s", rawMeta.Meta.Mode)
	}

	return &info, nil
}
