package storage

import (
	"context"
	"fmt"

	"github.com/hashicorp/go-version"
	"github.com/jmoiron/sqlx"
	log "github.com/sirupsen/logrus"
)

// DeviceProfile represents a row in the device_profile table
type DeviceProfile struct {
	ProfileId       int    `db:"profileid"`
	Region          string `db:"region"`
	MacVersion      string `db:"macversion"`
	RegionParameter string `db:"regionparameter"`
}

// Device represents a row in the device table
type Device struct {
	DeviceId        int    `db:"deviceid"`
	DeviceCode      string `db:"devicecode"`
	ModelId         int    `db:"modelid"`
	ProfileId       int    `db:"profileid"`
	FirmwareVersion string `db:"firmwareversion"`
	Status          int    `db:"status"`
}

// GetDevicesByModelAndVersion returns devices with the given modelId and a firmwareVersion less than the given version
func GetDevicesByModelAndVersion(ctx context.Context, db sqlx.Queryer, modelId int, maxVersion string) ([]Device, error) {
	var devices []Device

	// Query devices with the specified modelId
	query := `
		SELECT deviceid, devicecode, modelid, profileid, firmwareversion, status
		FROM device
		WHERE modelid = $1
	`
	if err := sqlx.Select(db, &devices, query, modelId); err != nil {
		return nil, fmt.Errorf("sql select error: %w", err)
	}

	// Parse the maxVersion string to a version object
	vMaxVersion, err := version.NewVersion(maxVersion)
	if err != nil {
		return nil, fmt.Errorf("invalid max version: %w", err)
	}

	// Filter devices with firmwareVersion less than maxVersion
	var filteredDevices []Device
	for _, device := range devices {
		vFirmwareVersion, err := version.NewVersion(device.FirmwareVersion)
		if err != nil {
			log.Printf("invalid firmware version: %s, skipping device %d", device.FirmwareVersion, device.DeviceId)
			continue
		}
		if vFirmwareVersion.LessThan(vMaxVersion) {
			filteredDevices = append(filteredDevices, device)
		}
	}

	return filteredDevices, nil
}

// GetRegionByDeviceId returns the region from profile id
func GetRegionByDeviceId(ctx context.Context, db sqlx.Queryer, deviceId int) (string, error) {

	var region string
	query := `
		SELECT region
		FROM device_profile
		WHERE profileId = (
			SELECT profileId
			FROM device
			WHERE deviceId = $1
		);
	`
	err := sqlx.Get(db, &region, query, deviceId)
	if err != nil {
		return region, fmt.Errorf("sql select error: %w", err)
	}

	return region, nil
}

func UpdateDeviceStatus(ctx context.Context, db sqlx.Execer, deviceId int, status int) error {

	res, err := db.Exec(`
		update device set
			status = $1,
		where
			deviceId = $2`,
		status,
		deviceId,
	)
	if err != nil {
		return fmt.Errorf("sql update error: %w", err)
	}
	ra, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("get rows affected error: %w", err)
	}
	if ra == 0 {
		return ErrDoesNotExist
	}

	log.WithFields(log.Fields{
		"deviceId": deviceId,
		"status":   status,
	}).Info("storage: device status updated")

	return nil
}

func UpdateDeviceFirmwareVersion(ctx context.Context, db sqlx.Execer, deviceId int, firmwareVersion string) error {

	res, err := db.Exec(`
		update device set
			firmwareVersion = $1,
		where
			deviceId = $2`,
		firmwareVersion,
		deviceId,
	)
	if err != nil {
		return fmt.Errorf("sql update error: %w", err)
	}
	ra, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("get rows affected error: %w", err)
	}
	if ra == 0 {
		return ErrDoesNotExist
	}

	log.WithFields(log.Fields{
		"deviceId":        deviceId,
		"firmwareVersion": firmwareVersion,
	}).Info("storage: device status updated")

	return nil
}
