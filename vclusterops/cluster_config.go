/*
 (c) Copyright [2023] Open Text.
 Licensed under the Apache License, Version 2.0 (the "License");
 You may not use this file except in compliance with the License.
 You may obtain a copy of the License at

 http://www.apache.org/licenses/LICENSE-2.0

 Unless required by applicable law or agreed to in writing, software
 distributed under the License is distributed on an "AS IS" BASIS,
 WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 See the License for the specific language governing permissions and
 limitations under the License.
*/

package vclusterops

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/vertica/vcluster/vclusterops/util"
	"github.com/vertica/vcluster/vclusterops/vlog"
	"gopkg.in/yaml.v3"
)

const (
	ConfigDirPerm  = 0755
	ConfigFilePerm = 0600
)

const ConfigFileName = "vertica_cluster.yaml"
const ConfigBackupName = "vertica_cluster.yaml.backup"

// VER-89599: add config file version
type ClusterConfig map[string]DatabaseConfig

type DatabaseConfig struct {
	Nodes                   []*NodeConfig `yaml:"nodes"`
	IsEon                   bool          `yaml:"eon_mode"`
	CommunalStorageLocation string        `yaml:"communal_storage_location"`
	Ipv6                    bool          `yaml:"ipv6"`
}

type NodeConfig struct {
	Name        string `yaml:"name"`
	Address     string `yaml:"address"`
	Subcluster  string `yaml:"subcluster"`
	CatalogPath string `yaml:"catalog_path"`
	DataPath    string `yaml:"data_path"`
	DepotPath   string `yaml:"depot_path"`
}

func MakeClusterConfig() ClusterConfig {
	return make(ClusterConfig)
}

func MakeDatabaseConfig() DatabaseConfig {
	return DatabaseConfig{}
}

// read config information from the YAML file
func ReadConfig(configDirectory string, log vlog.Printer) (ClusterConfig, error) {
	clusterConfig := ClusterConfig{}

	configFilePath := filepath.Join(configDirectory, ConfigFileName)
	configBytes, err := os.ReadFile(configFilePath)
	if err != nil {
		return clusterConfig, fmt.Errorf("fail to read config file, details: %w", err)
	}

	err = yaml.Unmarshal(configBytes, &clusterConfig)
	if err != nil {
		return clusterConfig, fmt.Errorf("fail to unmarshal config file, details: %w", err)
	}

	log.PrintInfo("The content of cluster config: %+v\n", clusterConfig)
	return clusterConfig, nil
}

// write config information to the YAML file
func (c *ClusterConfig) WriteConfig(configFilePath string) error {
	configBytes, err := yaml.Marshal(&c)
	if err != nil {
		return fmt.Errorf("fail to marshal config data, details: %w", err)
	}
	err = os.WriteFile(configFilePath, configBytes, ConfigFilePerm)
	if err != nil {
		return fmt.Errorf("fail to write config file, details: %w", err)
	}

	return nil
}

// GetPathPrefix returns catalog, data, and depot prefixes
func (c *ClusterConfig) GetPathPrefix(dbName string) (catalogPrefix string,
	dataPrefix string, depotPrefix string, err error) {
	dbConfig, ok := (*c)[dbName]
	if !ok {
		return "", "", "", cannotFindDBFromConfigErr(dbName)
	}

	if len(dbConfig.Nodes) == 0 {
		return "", "", "",
			fmt.Errorf("no node was found from the config file of %s", dbName)
	}

	return dbConfig.Nodes[0].CatalogPath, dbConfig.Nodes[0].DataPath,
		dbConfig.Nodes[0].DepotPath, nil
}

func (c *DatabaseConfig) GetHosts() []string {
	var hostList []string

	for _, vnode := range c.Nodes {
		hostList = append(hostList, vnode.Address)
	}

	return hostList
}

func GetConfigFilePath(dbName string, inputConfigDir *string, log vlog.Printer) (string, error) {
	var configParentPath string

	// if the input config directory is given and has write permission,
	// write the YAML config file under this directory
	if inputConfigDir != nil {
		if err := os.MkdirAll(*inputConfigDir, ConfigDirPerm); err != nil {
			return "", fmt.Errorf("fail to create config path at %s, detail: %w", *inputConfigDir, err)
		}

		return filepath.Join(*inputConfigDir, ConfigFileName), nil
	}

	// otherwise write it under the current directory
	// as <current_dir>/vertica_cluster.yaml
	currentDir, err := os.Getwd()
	if err != nil {
		log.Info("Fail to get current directory\n")
		configParentPath = currentDir
	}

	// create a directory with the database name
	// then write the config content inside this directory
	configDirPath := filepath.Join(configParentPath, dbName)
	if err := os.MkdirAll(configDirPath, ConfigDirPerm); err != nil {
		return "", fmt.Errorf("fail to create config path at %s, detail: %w", configDirPath, err)
	}

	configFilePath := filepath.Join(configDirPath, ConfigFileName)
	return configFilePath, nil
}

func BackupConfigFile(configFilePath string, log vlog.Printer) error {
	if util.CanReadAccessDir(configFilePath) == nil {
		// copy file to vertica_cluster.yaml.backup
		configDirPath := filepath.Dir(configFilePath)
		configFileBackup := filepath.Join(configDirPath, ConfigBackupName)
		log.Info("Config file exists and, creating a backup", "config file", configFilePath,
			"backup file", configFileBackup)
		err := util.CopyFile(configFilePath, configFileBackup, ConfigFilePerm)
		if err != nil {
			return err
		}
	}

	return nil
}

func RemoveConfigFile(configDirectory string, log vlog.Printer) error {
	configFilePath := filepath.Join(configDirectory, ConfigFileName)
	configBackupPath := filepath.Join(configDirectory, ConfigBackupName)

	err := os.RemoveAll(configFilePath)
	if err != nil {
		log.PrintError("Fail to remove the config file %s, detail: %s", configFilePath, err)
		return err
	}

	err = os.RemoveAll(configBackupPath)
	if err != nil {
		log.PrintError("Fail to remove the backup config file %s, detail: %s", configBackupPath, err)
		return err
	}

	return nil
}
