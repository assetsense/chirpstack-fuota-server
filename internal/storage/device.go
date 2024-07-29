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
		SELECT devicecode, modelid, profileid, firmwareversion, status
		FROM chirpstack.device
		WHERE modelid = $1 and status = 4270 and firmwareUpdateFailed = False and attempts < 3
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
			log.Errorf("invalid firmware version: %s, skipping device %s", device.FirmwareVersion, device.DeviceCode)
			continue
		}
		if vFirmwareVersion.LessThan(vMaxVersion) {
			filteredDevices = append(filteredDevices, device)
		}
	}

	return filteredDevices, nil
}

// GetRegionByDeviceCode returns the region from profile id
func GetRegionByDeviceCode(ctx context.Context, db sqlx.Queryer, deviceCode string) (string, error) {

	var region string
	query := `
		SELECT region
		FROM chirpstack.device_profile
		WHERE profileId = (
			SELECT profileId
			FROM chirpstack.device
			WHERE deviceCode = $1
		);
	`
	err := sqlx.Get(db, &region, query, deviceCode)
	if err != nil {
		return region, fmt.Errorf("sql select error: %w", err)
	}

	return region, nil
}

func UpdateDeviceStatus(ctx context.Context, db sqlx.Execer, deviceCode string, status int) error {

	res, err := db.Exec(`
		update chirpstack.device set
			status = $1,
		where
			deviceCode = $2`,
		status,
		deviceCode,
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
		"deviceCode": deviceCode,
		"status":     status,
	}).Info("storage: device status updated")

	return nil
}

func GetDeviceFirmwareUpdateFailed(ctx context.Context, db sqlx.Queryer, deviceCode string) (bool, error) {

	var failed bool
	query := `
		SELECT firmwareUpdateFailed
		FROM chirpstack.device
		WHERE deviceCode = $1
	`
	err := sqlx.Get(db, &failed, query, deviceCode)
	if err != nil {
		return failed, fmt.Errorf("sql select error: %w", err)
	}

	return failed, nil
}

func UpdateDeviceFirmwareUpdateFailedToTrue(ctx context.Context, db sqlx.Execer, deviceCode string) error {

	res, err := db.Exec(`
		update chirpstack.device set
			firmwareUpdateFailed = True, attempts = 0
		where
			deviceCode = $1 and attempts >= 2`,
		deviceCode,
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
		"deviceCode": deviceCode,
	}).Info("storage: device FirmwareUpdateFailed set to True")

	return nil
}

func IncrementDeviceAttempt(ctx context.Context, db sqlx.Execer, deviceCode string) error {

	res, err := db.Exec(`
		update chirpstack.device set
			attempts = attempts + 1
		where
			deviceCode = $1 and firmwareUpdateFailed = False`,
		deviceCode,
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

	UpdateDeviceFirmwareUpdateFailedToTrue(ctx, db, deviceCode)

	log.WithFields(log.Fields{
		"deviceCode": deviceCode,
	}).Info("storage: device attempts incremented")

	return nil
}

func UpdateDeviceFirmwareVersion(ctx context.Context, db sqlx.Execer, deviceCode string, firmwareVersion string) error {

	res, err := db.Exec(`
		update chirpstack.device set
			firmwareVersion = $1,
		where
			deviceCode = $2`,
		firmwareVersion,
		deviceCode,
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
		"deviceCode":      deviceCode,
		"firmwareVersion": firmwareVersion,
	}).Info("storage: device status updated")

	return nil
}
