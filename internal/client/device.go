package client

import (
    "database/sql"
    _ "github.com/lib/pq"
    "log"
)

type Device struct {
    DeviceId        int64
    IotModelId      int64
    ProfileId       int64
    FirmwareVersion string
    Region          int64
    MacVersion      int64
    RegionParameter int64
}

func getDevicesByModelId(db *sql.DB, iotModelId int64) ([]Device, error) {
    query := `SELECT * FROM device WHERE iotModelId = $1`
    rows, err := db.Query(query, iotModelId)
    if err != nil {
        return nil, err
    }
    defer rows.Close()

    var devices []Device
    for rows.Next() {
        var d Device
        if err := rows.Scan(&d.DeviceId, &d.IotModelId, &d.ProfileId, &d.FirmwareVersion, &d.Region, &d.MacVersion, &d.RegionParameter); err != nil {
            return nil, err
        }
        devices = append(devices, d)
    }
    return devices, nil
}
