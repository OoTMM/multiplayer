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
	Items         map[uint16]string
	Locations     map[uint32]string
}

type versionField struct {
	Version int `json:"version"`
}

func parseItems(reader *zip.ReadCloser, info *Info) {
	type itemEntry struct {
		ID  uint16 `json:"id"`
		Sym string `json:"sym"`
	}
	type itemsManifest struct {
		Items []itemEntry `json:"items"`
	}

	file, err := reader.Open("manifests/items.json")
	if err != nil {
		return
	}
	defer file.Close()
	data, err := io.ReadAll(file)
	if err != nil {
		return
	}
	var version versionField
	err = json.Unmarshal(data, &version)
	if err != nil {
		return
	}
	if version.Version != 1 {
		return
	}
	var manifest itemsManifest
	err = json.Unmarshal(data, &manifest)
	if err != nil {
		return
	}

	for _, item := range manifest.Items {
		info.Items[item.ID] = item.Sym
	}
}

func parseLocations(reader *zip.ReadCloser, info *Info) {
	type entry struct {
		Key      uint32 `json:"key"`
		Location string `json:"location"`
	}
	type manifest struct {
		Locations []entry `json:"locations"`
	}

	file, err := reader.Open("manifests/locations.json")
	if err != nil {
		return
	}
	defer file.Close()
	data, err := io.ReadAll(file)
	if err != nil {
		return
	}
	var version versionField
	err = json.Unmarshal(data, &version)
	if err != nil {
		return
	}
	if version.Version != 1 {
		return
	}
	var m manifest
	err = json.Unmarshal(data, &m)
	if err != nil {
		return
	}

	for _, loc := range m.Locations {
		info.Locations[loc.Key] = loc.Location
	}
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
	info.Items = make(map[uint16]string)
	info.Locations = make(map[uint32]string)
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

	parseItems(reader, &info)
	parseLocations(reader, &info)

	return &info, nil
}
