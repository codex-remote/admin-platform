package device

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"time"

	"github.com/ai-coding-remote/admin-platform/internal/model"
)

var validDeviceID = regexp.MustCompile(`^[A-Za-z0-9-]{8,64}$`)

type devicectlOutput struct {
	Result struct {
		Devices []struct {
			Identifier string `json:"identifier"`
			Connection struct {
				PairingState       string `json:"pairingState"`
				LastConnectionDate string `json:"lastConnectionDate"`
			} `json:"connectionProperties"`
			Device struct {
				Name            string `json:"name"`
				OSBuildUpdate   string `json:"osBuildUpdate"`
				OSVersionNumber string `json:"osVersionNumber"`
				ProductType     string `json:"productType"`
			} `json:"deviceProperties"`
			Hardware struct {
				MarketingName string `json:"marketingName"`
				ProductType   string `json:"productType"`
				UDID          string `json:"udid"`
			} `json:"hardwareProperties"`
		} `json:"devices"`
	} `json:"result"`
}

func List(ctx context.Context) ([]model.Device, error) {
	temp, err := os.CreateTemp("", "codexremote-devices-*.json")
	if err != nil {
		return nil, err
	}
	path := temp.Name()
	temp.Close()
	defer os.Remove(path)
	cmd := exec.CommandContext(ctx, "xcrun", "devicectl", "list", "devices", "--json-output", path, "--quiet")
	if output, err := cmd.CombinedOutput(); err != nil {
		return nil, fmt.Errorf("devicectl: %s: %w", output, err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var result devicectlOutput
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, err
	}
	devices := make([]model.Device, 0, len(result.Result.Devices))
	for _, d := range result.Result.Devices {
		id := d.Hardware.UDID
		if id == "" {
			id = d.Identifier
		}
		devices = append(devices, model.Device{ID: id, Name: d.Device.Name, MarketingName: d.Hardware.MarketingName,
			OSVersion: d.Device.OSVersionNumber, OSBuild: d.Device.OSBuildUpdate, ProductType: d.Hardware.ProductType,
			Connected: d.Connection.LastConnectionDate != "", Paired: d.Connection.PairingState == "paired", Platform: "iphoneos"})
	}
	return devices, nil
}

func SafeList(ctx context.Context) ([]model.Device, error) {
	devices, err := List(ctx)
	if err != nil {
		return nil, err
	}
	for index := range devices {
		devices[index].ID = Reference(devices[index].ID)
	}
	return devices, nil
}

func Resolve(ctx context.Context, reference string) (string, error) {
	devices, err := List(ctx)
	if err != nil {
		return "", err
	}
	for _, candidate := range devices {
		if Reference(candidate.ID) == reference {
			return candidate.ID, nil
		}
	}
	return "", fmt.Errorf("device reference is not currently connected")
}

func Reference(identifier string) string {
	sum := sha256.Sum256([]byte("codexremote-device-v1\x00" + identifier))
	return fmt.Sprintf("device_%x", sum[:12])
}

func Sysdiagnose(ctx context.Context, deviceID, destination string, fullLogs bool) error {
	if !validDeviceID.MatchString(deviceID) {
		return fmt.Errorf("invalid device identifier")
	}
	if err := os.MkdirAll(destination, 0o700); err != nil {
		return err
	}
	args := []string{"devicectl", "device", "sysdiagnose", "--device", deviceID, "--destination", destination, "--timeout", "900"}
	if fullLogs {
		args = append(args, "--gather-full-logs")
	}
	cmd := exec.CommandContext(ctx, "xcrun", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("sysdiagnose: %s: %w", output, err)
	}
	return nil
}

func NewestArtifact(root string) (string, error) {
	var newest string
	var newestTime time.Time
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if entry.IsDir() {
			return nil
		}
		info, err := entry.Info()
		if err == nil && info.ModTime().After(newestTime) {
			newest = path
			newestTime = info.ModTime()
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	if newest == "" {
		return "", fmt.Errorf("sysdiagnose produced no artifact")
	}
	return newest, nil
}
