package storage

import (
	"database/sql"
	"encoding/json"
	"errors"
	"github.com/google/uuid"
	_ "github.com/mattn/go-sqlite3"
	"go.uber.org/zap"
	"golang.org/x/exp/slices"
)

type sqliteStorage struct {
	logger *zap.Logger
	db     *sql.DB
}

func NewSqliteStorage(logger *zap.Logger) (Service, error) {
	db, err := startSqliteDb(logger)
	if err != nil {
		return nil, err
	}
	return sqliteStorage{db: db, logger: logger}, nil
}

func (s sqliteStorage) GetInterfaceByPolicyAndNamespace(policy, namespace string) ([]DbInterface, error) {
	selectResult, err := s.db.Query(`
		SELECT (id, policy, namespace, hostname, name, admin_state, mtu, speed, mac_address, if_type, json_data) 
		FROM interfaces
		WHERE policy = $1 AND namespace = $2
	`, policy, namespace)
	if err != nil {
		s.logger.Error("failed to fetch interface", zap.Error(err))
		return nil, err
	}
	var interfaces []DbInterface
	for selectResult.Next() {
		var iface DbInterface
		err := selectResult.Scan(&iface.Id, &iface.Policy, &iface.Namespace, &iface.Hostname, &iface.Name, &iface.AdminState,
			&iface.Mtu, &iface.Speed, &iface.MacAddress, &iface.IfType, &iface.Blob)
		if err != nil {
			s.logger.Error("failed to create iface struct", zap.Error(err))
			return nil, err
		}
		interfaces = append(interfaces, iface)
	}

	return interfaces, nil
}

func (s sqliteStorage) GetDevicesByPolicyAndNamespace(policy, namespace string) ([]DbDevice, error) {
	selectResult, err := s.db.Query(`
		SELECT (id, policy, namespace, hostname, serial_number, model, state, vendor, json_data) 
		FROM devices
		WHERE policy = $1 AND namespace = $2
	`, policy, namespace)
	if err != nil {
		s.logger.Error("failed to fetch device", zap.Error(err))
		return nil, err
	}
	var devices []DbDevice
	for selectResult.Next() {
		var device DbDevice
		err := selectResult.Scan(&device.Id, &device.Policy, &device.Namespace, &device.Hostname, &device.SerialNumber,
			&device.Model, &device.State, &device.Vendor, &device.Blob)
		if err != nil {
			s.logger.Error("failed to create device struct", zap.Error(err))
			return nil, err
		}
		devices = append(devices, device)
	}

	return devices, nil
}

func (s sqliteStorage) GetVlansByPolicyAndNamespace(policy, namespace string) ([]DbVlan, error) {
	selectResult, err := s.db.Query(`
		SELECT (id, policy, namespace, hostname, name, state, json_data) 
		FROM vlans
		WHERE policy = $1 AND namespace = $2
	`, policy, namespace)
	if err != nil {
		s.logger.Error("failed to fetch device", zap.Error(err))
		return nil, err
	}
	var vlans []DbVlan
	for selectResult.Next() {
		var vlan DbVlan
		err := selectResult.Scan(&vlan.Id, &vlan.Policy, &vlan.Namespace, &vlan.Hostname, &vlan.Name,
			&vlan.State, &vlan.Blob)
		if err != nil {
			s.logger.Error("failed to create vlan struct", zap.Error(err))
			return nil, err
		}
		vlans = append(vlans, vlan)
	}

	return vlans, nil
}

func (s sqliteStorage) Save(policy string, jsonData map[string]interface{}) (stored interface{}, err error) {
	ifData, ok := jsonData["interfaces"].([]interface{})
	if ok {
		interfacesAdded := make([]DbInterface, len(ifData))
		for _, interfaceData := range ifData {
			dataAsString, err := json.Marshal(interfaceData)
			if err != nil {
				s.logger.Error("error marshalling interface data", zap.Error(err))
				continue
			}
			dbInterface := DbInterface{
				Id:     uuid.NewString(),
				Policy: policy,
				Blob:   string(dataAsString),
			}
			err = json.Unmarshal(dataAsString, &dbInterface)
			if err != nil {
				s.logger.Error("error marshalling interface data", zap.Error(err))
				continue
			}
			statement, err := s.db.Prepare(`
			INSERT INTO interfaces 
			    (
			     id, 
				 policy, 
				 namespace,
				 hostname,
				 name,
				 admin_state,
				 mtu,
				 speed,
				 mac_address,
				 if_type, 
				 json_data
			    ) VALUES 
				  (
				   $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11
				  )
			`)
			if err != nil {
				s.logger.Error("error during preparing insert statement on interface", zap.Error(err))
				return nil, err
			}
			_, err = statement.Exec(dbInterface.Id, policy, dbInterface.Namespace, dbInterface.Hostname, dbInterface.Name,
				dbInterface.AdminState, dbInterface.Mtu, dbInterface.Speed, dbInterface.MacAddress, dbInterface.IfType, dataAsString)
			if err != nil {
				s.logger.Error("error during preparing insert statement on interface", zap.Error(err))
				return nil, err
			}
			interfacesAdded = append(interfacesAdded, dbInterface)
		}
		if err != nil {
			return nil, err
		}
		return interfacesAdded, nil
	}
	dData, ok := jsonData["device"].([]interface{})
	if ok {
		devicesAdded := make([]DbDevice, len(dData))
		for _, deviceData := range dData {
			dataAsString, err := json.Marshal(deviceData)
			if err != nil {
				s.logger.Error("error marshalling interface data", zap.Error(err))
				return nil, err
			}
			dbDevice := DbDevice{
				Id:     uuid.NewString(),
				Policy: policy,
				Blob:   string(dataAsString),
			}
			err = json.Unmarshal(dataAsString, &dbDevice)
			if err != nil {
				s.logger.Error("error marshalling interface data", zap.Error(err))
				return nil, err
			}
			statement, err := s.db.Prepare(
				`
				INSERT INTO devices 
					(
					id,
					policy, 
					namespace,
					hostname,
					address,
					serial_number,
					model,
					state,
					vendor, 
					json_data) 
				VALUES 
					(
					  $1, $2, $3, $4, $5, $6, $7, $8, $9, $10
					)
		`)
			if err != nil {
				s.logger.Error("error during preparing insert statement", zap.Error(err))
				return nil, err
			}
			_, err = statement.Exec(dbDevice.Id, policy, dbDevice.Namespace, dbDevice.Hostname, dbDevice.Address, dbDevice.SerialNumber,
				dbDevice.Model, dbDevice.State, dbDevice.Vendor, dataAsString)
			if err != nil {
				s.logger.Error("error during executing insert statement", zap.Error(err))
				return nil, err
			}
			devicesAdded = append(devicesAdded, dbDevice)
		}
		return devicesAdded, nil
	}
	vData, ok := jsonData["vlan"].([]interface{})
	if ok {
		vlans := make([]DbVlan, len(vData))
		for _, vlanData := range vData {
			dataAsString, err := json.Marshal(vlanData)
			if err != nil {
				s.logger.Error("error marshalling interface data", zap.Error(err))
				return nil, err
			}
			vlan := DbVlan{
				Id:     uuid.NewString(),
				Policy: policy,
				Blob:   string(dataAsString),
			}
			err = json.Unmarshal(dataAsString, &vlan)
			if err != nil {
				s.logger.Error("error marshalling interface data", zap.Error(err))
				return nil, err
			}
			statement, err := s.db.Prepare(
				`
				INSERT INTO vlans 
					(
						id,
						policy,
						namespace,
						hostname,
						name,
						state,
						json_data
					)
				VALUES 
					(
					  $1, $2, $3, $4, $5, $6, $7
					)
		`)
			if err != nil {
				s.logger.Error("error during preparing insert statement", zap.Error(err))
				return nil, err
			}
			_, err = statement.Exec(vlan.Id, policy, vlan.Namespace, vlan.Hostname, vlan.Name,
				vlan.State, dataAsString)
			if err != nil {
				s.logger.Error("error during executing insert statement", zap.Error(err))
				return nil, err
			}
			vlans = append(vlans, vlan)
		}
		return vlans, nil
	}
	return nil, errors.New("not able to save anything from entry")
}

func startSqliteDb(logger *zap.Logger) (db *sql.DB, err error) {
	if !slices.Contains(sql.Drivers(), "sqlite3") {
		logger.Error("SQLite does not have required driver", zap.Error(err))
		return nil, err
	}
	db, err = sql.Open("sqlite3", ":memory")
	if err != nil {
		logger.Error("SQLite could not be initialized", zap.Error(err))
		return nil, err
	}

	createInterfacesTableStatement, err := db.Prepare(`
		CREATE TABLE IF NOT EXISTS interfaces 
		( id TEXT PRIMARY KEY, 
		 policy TEXT, 
		 namespace TEXT,
		 hostname TEXT,
		 name TEXT,
		 admin_state TEXT,
		 mtu INTEGER,
		 speed INTEGER,
		 mac_address TEXT,
		 if_type TEXT,
		 netbox_id INTEGER NULL, 
		 json_data TEXT )
    `)
	if err != nil {
		logger.Error("error preparing interfaces statement", zap.Error(err))
		return nil, err
	}
	_, err = createInterfacesTableStatement.Exec()
	if err != nil {
		logger.Error("error creating interfaces table", zap.Error(err))
		return nil, err
	}
	logger.Debug("successfully created Interfaces table")
	createDeviceTableStatement, err := db.Prepare(`
		CREATE TABLE IF NOT EXISTS devices 
		(
		    id TEXT PRIMARY KEY, 
		 	policy TEXT, 
		 	namespace TEXT,
		 	hostname TEXT,
		 	serial_number TEXT,
		 	address TEXT,
		 	model TEXT,
		 	state TEXT,
		 	vendor TEXT,
		 	netbox_id INTEGER NULL, 
		    json_data TEXT 
		)
    `)
	if err != nil {
		logger.Error("error preparing devices statement ", zap.Error(err))
		return nil, err
	}
	_, err = createDeviceTableStatement.Exec()
	if err != nil {
		logger.Error("error creating devices table", zap.Error(err))
		return nil, err
	}
	logger.Debug("successfully created devices table")

	createVlansTableStatement, err := db.Prepare(`
	CREATE TABLE IF NOT EXISTS vlans
	(
	    id TEXT PRIMARY KEY,
	    policy TEXT,
	    namespace TEXT,
		hostname TEXT,
		name TEXT,
		state TEXT,
		netbox_id INTEGER NULL,
		json_data TEXT 
	)`)
	if err != nil {
		logger.Error("error preparing vlans statement ", zap.Error(err))
		return nil, err
	}
	_, err = createVlansTableStatement.Exec()
	if err != nil {
		logger.Error("error creating vlans table", zap.Error(err))
		return nil, err
	}
	constraint1TableStatement, err := db.Prepare(`
		CREATE UNIQUE INDEX interfaces_uniques ON interfaces(policy, namespace, hostname, name)
	`)
	if err != nil {
		logger.Error("error constraints statement ", zap.Error(err))
		return nil, err
	}
	_, err = constraint1TableStatement.Exec()
	if err != nil {
		logger.Error("error constraints execution", zap.Error(err))
		return nil, err
	}
	constraint2TableStatement, err := db.Prepare(`
		CREATE UNIQUE INDEX devices_uniques ON devices(policy, namespace, hostname, address)
	`)
	if err != nil {
		logger.Error("error constraints statement ", zap.Error(err))
		return nil, err
	}
	_, err = constraint2TableStatement.Exec()
	if err != nil {
		logger.Error("error constraints execution", zap.Error(err))
		return nil, err
	}
	constraint3TableStatement, err := db.Prepare(`
		CREATE UNIQUE INDEX vlans_uniques ON vlans(policy, namespace, hostname, name)
	`)
	if err != nil {
		logger.Error("error constraints statement ", zap.Error(err))
		return nil, err
	}
	_, err = constraint3TableStatement.Exec()
	if err != nil {
		logger.Error("error constraints execution", zap.Error(err))
		return nil, err
	}

	return
}
