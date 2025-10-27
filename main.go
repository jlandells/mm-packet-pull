// Package main is a standalone utility that assists in gatrhering useful data for Mattermost Support from a
// non-running Mattermost instance.
package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strings"
	"time"
)

// Defaults & Type Definitions

var debugMode bool = false
var osPlatform string = ""

// LogLevel is used to refer to the type of message that will be written using the logging code.
type LogLevel string

// Creating this as a struct already in case we need to extract additional items from the config file
type mmConfig struct {
	LogDirectory string
	ListenPort   string
}

const (
	defaultMattermostDir = "/opt/mattermost"
	defaultPacketProfix  = "support-packet"
	defaultTargetDir     = "/tmp"
	defaultListenPort    = "8065"
)

const (
	debugLevel   LogLevel = "DEBUG"
	infoLevel    LogLevel = "INFO"
	warningLevel LogLevel = "WARNING"
	errorLevel   LogLevel = "ERROR"
)

const ()

// Logging functions

// LogMessage logs a formatted message to stdout or stderr
func LogMessage(level LogLevel, message string) {
	if level == errorLevel {
		log.SetOutput(os.Stderr)
	} else {
		log.SetOutput(os.Stdout)
	}
	log.SetFlags(log.Ldate | log.Ltime)
	log.Printf("[%s] %s\n", level, message)
}

// DebugPrint allows us to add debug messages into our code, which are only printed if we're running in debug more.
// Note that the command line parameter '-debug' can be used to enable this at runtime.
func DebugPrint(message string) {
	if debugMode {
		LogMessage(debugLevel, message)
	}
}

// Utility Functions

// isRoot returns true of the program is being executed with root priveleges, otherwise it returns false.
func isRoot() bool {
	currentUser, err := user.Current()
	if err != nil {
		log.Fatalf("[isRoot] Unable to get current user: %s", err)
	}
	return currentUser.Username == "root"
}

// getEnvWithDefaults allows us to retrieve Environment variables, and to return either the current value or a supplied default
func getEnvWithDefault(key string, defaultValue interface{}) interface{} {
	value, exists := os.LookupEnv(key)
	if !exists {
		return defaultValue
	}
	return value
}

// fileExists is a utility function to validate that a file exists and is not a directory.  Returns true/false.
func fileExists(filename string) bool {
	info, err := os.Stat(filename)
	if os.IsNotExist(err) {
		LogMessage(warningLevel, "File '"+filename+"' does not exist!")
		return false
	}
	if info.IsDir() {
		LogMessage(warningLevel, "File '"+filename+"' is a directory!")
		return false
	}
	return true
}

// dirExists is a utility function that checks that a directory exists and that it is truly a directory.  Returns true/false.
func dirExists(dirname string) bool {
	info, err := os.Stat(dirname)
	if os.IsNotExist(err) {
		LogMessage(warningLevel, "Directory '"+dirname+"' does not exist!")
		return false
	}
	if !info.IsDir() {
		LogMessage(warningLevel, "Directory '"+dirname+"' is not a directory!")
		return false
	}
	return true
}

// checkPackage is used to check whether a utility is available on the current linux distro in use.
// In many cases, the command to be checked for (passed in as a parameter) is its own package, but in
// a few special cases, commands exist as part of a larger suite.  For these cases, we need to maintain
// a map of the specific cases (see commandPkgMap below) to allow us to identify what package the command
// in question is part of for a particular distro.  Returns true if the command is available, otherwise false.
//
// LIMITATIONS:  Currently only written to handle Ubuntu, CentOS and Fedora.  Additional development would be
// required to add more Linux distros.
func checkPackage(command string) bool {
	var cmd *exec.Cmd
	var packageName string

	if osPlatform == "" {
		cmd = exec.Command("bash", "-c", "cat /etc/*-release | grep '^ID='")
		var out bytes.Buffer
		cmd.Stdout = &out
		err := cmd.Run()
		if err != nil {
			LogMessage(errorLevel, "Unable to determine OS!")
			return false
		}
		distroInfo := strings.Split(out.String(), "=")
		if len(distroInfo) > 1 {
			osPlatform = strings.TrimSpace(distroInfo[1])
		}
		DebugPrint("Running on " + osPlatform)
	}

	// Define a map for package names based on distribution and command
	commandPkgMap := map[string]map[string]string{
		"ubuntu": {
			"ss":      "iproute2",
			"netstat": "net-tools",
		},
		"centos": {
			"ss":      "iproute",
			"netstat": "net-tools",
		},
		"fedora": {
			"ss":      "iproute",
			"netstat": "net-tools",
		},
	}

	if val, ok := commandPkgMap[osPlatform][command]; ok {
		packageName = val
	} else {
		LogMessage(warningLevel, "Package not found for "+command+". Testing using command name directly.")
		packageName = command
	}

	switch osPlatform {
	case "ubuntu":
		cmd = exec.Command("dpkg", "-l", packageName)
	case "centos":
		cmd = exec.Command("yum", "list", "installed", packageName)
	case "fedora":
		cmd = exec.Command("dnf", "list", "installed", packageName)
	default:
		LogMessage(errorLevel, "We should never get here!")
		return false
	}

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		LogMessage(warningLevel, fmt.Sprint(err)+": "+stderr.String())
		return false
	}

	return true
}

// Task specific functions

// ProcessConfigFile is use to extract key information from the Mattermost config file (usually config.json)
// and to store that information in specific values in a custom struct (MMConfig).  Note that the struct was
// used to make it simplye to expand, whilst offering the flexibility of passing the entire structure to
// any functions that might need it.
func (confFile *mmConfig) ProcessConfigFile(configPath string, mmDir string) error {
	DebugPrint("Processing config file: " + configPath)

	file, err := os.Open(configPath)
	if err != nil {
		LogMessage(errorLevel, "Failed to open config file!")
		return errors.New("failed to open config file")
	}
	defer file.Close()

	byteValue, _ := io.ReadAll(file)

	// Declare an empty interface
	var result map[string]interface{}

	// Unmarshal the byte slice into the empty interface
	json.Unmarshal([]byte(byteValue), &result)

	// Extract the log file directory
	if logSettings, ok := result["LogSettings"].(map[string]interface{}); ok {
		if fileLocation, ok := logSettings["FileLocation"].(string); ok {
			if fileLocation == "" {
				confFile.LogDirectory = mmDir + "/logs"
				LogMessage(infoLevel, "No logs directory override in config.json.  Using defaults.")
			} else {
				confFile.LogDirectory = fileLocation
				LogMessage(infoLevel, "Using log directory from config file: "+confFile.LogDirectory)
			}
		}
	}

	if !dirExists(confFile.LogDirectory) {
		return errors.New("specified log directory does exist")
	}

	// Extract the listen port
	if serviceSettings, ok := result["ServiceSettings"].(map[string]interface{}); ok {
		if listenPort, ok := serviceSettings["ListenAddress"].(string); ok {
			if listenPort == "" {
				LogMessage(warningLevel, "No listen port found in config file!  Defaulting to: "+defaultListenPort)
				confFile.ListenPort = defaultListenPort
			} else {
				lastColonIndex := strings.LastIndex(listenPort, ":")
				if lastColonIndex == -1 {
					confFile.ListenPort = listenPort
				} else {
					confFile.ListenPort = listenPort[lastColonIndex+1:]
				}
				LogMessage(infoLevel, "Using listen port from config file: "+confFile.ListenPort)
			}
		}
	}

	return nil

}

// createTempDir creates a temporary directory into which the files to be included in the support packet
// will be collected.  It is this directory that will ultimately be compressed to share with Mattermost.
// The function takes as parameters the parent directory into which the temp directory will be places,
// and a prefix.  This prefix can be modified using the -name command line parameter, and will have the
// current timestamp appended to name the directory.  This timestampped name will ultimately be used as
// the name for the compressed file.
// The function returns the full path to the directory if successful, and an error object.  If the directory
// is successfully created, the error object will be nil, otherwise the path will be an empty string and
// the error object will be populated.
func createTempDir(targetDir string, namePrefix string) (string, error) {
	DebugPrint("Creating temp directory in '" + targetDir + "' with prefix: " + namePrefix)

	currentTime := time.Now()
	timeString := currentTime.Format("2006-01-02_15-04-05")

	// Combine the path, name prefix and time string to give the full path
	dirName := fmt.Sprintf("%s/%s_%s", targetDir, namePrefix, timeString)

	DebugPrint("Full Path to temp directory calculated as: " + dirName)

	// Now we can create the directory, ready to receive the support packet
	err := os.MkdirAll(dirName, 0755)
	if err != nil {
		LogMessage(errorLevel, "Failed to create directory: "+dirName)
		return "", errors.New(err.Error())
	}

	return dirName, nil
}

// CopyLogFiles copies any files in the Mattermost log directory into the temp directory.  Both directories
// are passed as parameters.  The function returns an error object if it fails, otherwise it returns nil.
func CopyLogFiles(logFileDirectory string, targetDirectory string) error {
	DebugPrint("Copying files from:'" + logFileDirectory + "' to: '" + targetDirectory + "'")

	source := fmt.Sprintf("%s/*", logFileDirectory)
	target := fmt.Sprintf("%s/.", targetDirectory)

	DebugPrint("Copying from source: " + source + " to target: " + target)

	copyCommand := fmt.Sprintf("cp %s %s", source, target)

	DebugPrint("Copy command: " + copyCommand)

	// Note that the copy command requires wildcard substitution, which is handled by the underlying shell.
	// By default, the os/exec package does not start a shell so the wildcard expansion fails.  To rectify this,
	// we run the copy inside a `sh -c` shell, providing ourselves with a handy shell to handle the wildcards.
	cmd := exec.Command("sh", "-c", copyCommand)
	output, err := cmd.CombinedOutput()
	if err != nil {
		LogMessage(errorLevel, "Unable to copy files from:'"+logFileDirectory+"' to: '"+targetDirectory+"'. Error: "+err.Error()+" Output: "+string(output))
		return errors.New(err.Error())
	}

	return nil
}

// CopyConfigFile handles the copying of the Mattermost config file (usually config.json) to the temp directory.
// Paths to both the config file and the temp directory are passed as parameters, and an error object is returned
// on failure.
func CopyConfigFile(configFileName string, targetDirectory string) error {
	DebugPrint("Copying config file from: '" + configFileName + "' to '" + targetDirectory + "'")

	cmd := exec.Command("cp", configFileName, targetDirectory+"/.")
	err := cmd.Run()
	if err != nil {
		LogMessage(errorLevel, "Unable to copy config file '"+configFileName+"' to '"+targetDirectory+"'")
		return errors.New(err.Error())
	}

	return nil
}

// GatherServiceMessages is a function that allows us to obtain the output of the traiditional
// systemctl and journalctl messages that would typically be run on the command line when a service
// fails to start.  The temp directory is passed in as a parameter and we return a bool to indicate
// complete success (true) or failure of one or more steps (false).
// The information is written to systemctl.txt and journalctl.txt in the temp directory.
func GatherServiceMessages(targetDir string) bool {
	DebugPrint("Gathering service messages - writing to: " + targetDir)

	noErrors := true

	// We'll write the service logs to two text files: systemctl.txt & journalctl.txt
	sysFile, err := os.Create(targetDir + "/systemctl.txt")
	if err != nil {
		LogMessage(warningLevel, "Failed to create output file for systemctl output")
		noErrors = false
	} else {
		cmd := exec.Command("systemctl", "status", "mattermost.service", "--no-pager", "-l")
		cmd.Stdout = sysFile
		cmd.Stderr = sysFile

		err = cmd.Run()
		if err != nil {
			LogMessage(warningLevel, "Failed to generate output from systemctl: "+err.Error())
			noErrors = false
		}
	}
	defer sysFile.Close()

	jnlFile, err := os.Create(targetDir + "/journalctl.txt")
	if err != nil {
		LogMessage(warningLevel, "Failed to create output file for journalctl output")
		noErrors = false
	} else {
		cmd := exec.Command("journalctl", "-xe", "--no-pager")
		cmd.Stdout = jnlFile
		cmd.Stderr = jnlFile

		err = cmd.Run()
		if err != nil {
			LogMessage(warningLevel, "Failed to generate output from journalctl: "+err.Error())
			noErrors = false
		}
	}
	defer jnlFile.Close()

	return noErrors
}

// GetTopProcesses calls `top` in batch mode to gather a list of the top-running processes, just as
// we'd expect to see when running top inderactively.  The temp directory is passed as a parameter,
// and we return an error object.
// The information is written to top.txt in the temp directory.
func GetTopProcesses(targetDir string) error {
	DebugPrint("Gathering top processes - writing to: " + targetDir)

	file, err := os.Create(targetDir + "/top.txt")
	if err != nil {
		LogMessage(errorLevel, "Unable to create file for top processes in "+targetDir)
		return errors.New(err.Error())
	}
	defer file.Close()

	// The `-b` flag runs top in batch more, and `-n` allows us to specify the number of
	// iterations - in this case, we only want 1.
	cmd := exec.Command("top", "-b", "-n", "1")
	cmd.Stdout = file
	cmd.Stderr = file

	err = cmd.Run()
	if err != nil {
		LogMessage(warningLevel, "Failed to generate output from top")
		return errors.New(err.Error())
	}

	return nil
}

// CheckListeningPort uses either netstat or ss to see what processes (if any) are listening on the
// port that Mattermost is trying to use.  Note that we can't be sure which package is installed
// (if any) for these commands, so we need to figure that out first.
// THe function takes the port in question and the temp directory as parameters, and returns an
// error object (nil on success).
// The result is stored in portinfo.txt.
func CheckListeningPort(port string, targetDir string) error {
	DebugPrint("Checking for what's listening on port " + port)

	// We need to see whether we have `netstat` available, or if not, do we have `ss`?
	var cmdName string
	cmdArgs := fmt.Sprintf("-tulnp | grep %s", port)

	if checkPackage("netstat") {
		cmdName = "netstat"
	} else if checkPackage("ss") {
		cmdName = "ss"
	} else {
		LogMessage(errorLevel, "Neither netstat nor ss found!")
		return errors.New("mising package")
	}

	// Prepare the output file
	file, err := os.Create(targetDir + "/portinfo.txt")
	if err != nil {
		LogMessage(errorLevel, "Unable to create file for port information in "+targetDir)
		return errors.New(err.Error())
	}
	defer file.Close()

	// We build the full command in this manner due to needing to pipe the output through grep,
	// for which we'll need to run the command in a sub-shell
	fullCommand := fmt.Sprintf("%s %s", cmdName, cmdArgs)

	DebugPrint("Port info command: " + fullCommand)

	// Execute the command in a sub-shell
	cmd := exec.Command("sh", "-c", fullCommand)
	cmd.Stdout = file
	cmd.Stderr = file

	err = cmd.Run()
	if err != nil {
		LogMessage(warningLevel, "Failed to locate port information!")
		return errors.New(err.Error())
	}

	return nil
}

// CopyOSInfoFiles takes a copy of the os-release and meminfo files in the temp directory, in case these are
// useful for troubleshooting.  It takes a single parameter - the temp directory.  The function returns a boolean
// to indicate complete success (true), or false to indicate that one or more steps failed.
func CopyOSInfoFiles(targetDir string) bool {
	DebugPrint("Copying OS info files to " + targetDir)

	noErrors := true

	cmd := exec.Command("cp", "/etc/os-release", targetDir+"/.")

	err := cmd.Run()
	if err != nil {
		LogMessage(warningLevel, "Failed to copy os-release. Error "+err.Error())
		noErrors = false
	}

	cmd = exec.Command("cp", "/proc/meminfo", targetDir+"/.")

	err = cmd.Run()
	if err != nil {
		LogMessage(warningLevel, "Failed to copy meminfo.  Error: "+err.Error())
		noErrors = false
	}

	return noErrors
}

// GetDiskSpace uses the OS level `df -a -h` to provide disk space information across all disks in
// human readable form.  We expect the temp directory as a parameter, and return an error object on failure.
// The output is written to diskspace.txt
func GetDiskSpace(targetDir string) error {
	DebugPrint("Getting disk space")

	file, err := os.Create(targetDir + "/diskspace.txt")
	if err != nil {
		LogMessage(errorLevel, "Unable to create file for disk space in "+targetDir)
		return errors.New(err.Error())
	}
	defer file.Close()

	cmd := exec.Command("df", "-a", "-h")
	cmd.Stdout = file
	cmd.Stderr = file

	err = cmd.Run()
	if err != nil {
		LogMessage(warningLevel, "Failed to generate output from df")
		return errors.New(err.Error())
	}

	return nil
}

// CompressSupportPacket is used as the last step in the process, to take the directory containing all of the files
// (passed om as targetDir) and to compress them into a tar.gz file in the parent directory (passed in as parentDir).
// The function generates the name of the tar.gz file by taking the name of the temp directory and suffixing .tar.gz.
// The function returns the full path to the tar.gz file on success, as well as an error object (nil on success).
// If anything fails in this process, the path will be returned as a null string, and more information on the error
// will be contained in the error object.
func CompressSupportPacket(targetDir string, parentDir string) (string, error) {
	DebugPrint("Compressing temp directory: " + targetDir)
	DebugPrint("TAR file to be located in: " + parentDir)

	compressedFileNameBase := filepath.Base(targetDir)

	DebugPrint("compressedFileNameBase: " + compressedFileNameBase)

	compressedFileName := fmt.Sprintf("%s/%s.tar.gz", parentDir, compressedFileNameBase)

	DebugPrint("compressedFileName: " + compressedFileName)

	cmd := exec.Command("tar", "-cvzf", compressedFileName, targetDir)
	err := cmd.Run()
	if err != nil {
		LogMessage(errorLevel, "Failed to compress support packet!  Error: "+err.Error())
		return "", errors.New(err.Error())
	}

	return compressedFileName, nil
}

// Main section

func main() {

	// Check that user is running with root privileges - abort if not!
	if !isRoot() {
		LogMessage(errorLevel, "'root' or 'sudo' priveleges are required to run this utility!  Please try again using 'sudo'.")
		os.Exit(2)
	}

	// Parse command line
	var MattermostDir string
	var TargetDir string
	var PkgNamePrefix string
	var DebugFlag bool
	var NoObfuscateFlag bool

	flag.StringVar(&MattermostDir, "directory", "", "Install directory of Mattermost. [Default: "+defaultMattermostDir+"]")
	flag.StringVar(&TargetDir, "target", "", "Target directory in which the support packet will be created. [Default: "+defaultTargetDir+"]")
	flag.StringVar(&PkgNamePrefix, "name", "", "Prefix for name of support packet. [Default: "+defaultPacketProfix+"]")
	flag.BoolVar(&DebugFlag, "debug", false, "Enable debug mode.")
	flag.BoolVar(&NoObfuscateFlag, "no-obfuscate", false, "Disable obfuscation of sensitive data in logs and config files. [Default: obfuscation enabled]")

	flag.Parse()

	// If information not supplied on the command line, check whether it's available as an envrionment variable
	if MattermostDir == "" {
		MattermostDir = getEnvWithDefault("MM_SUP_DIR", defaultMattermostDir).(string)
	}
	if TargetDir == "" {
		TargetDir = getEnvWithDefault("MM_SUP_TGT", defaultTargetDir).(string)
	}
	if PkgNamePrefix == "" {
		PkgNamePrefix = getEnvWithDefault("MM_SUP_NAME", defaultPacketProfix).(string)
	}
	if !DebugFlag {
		DebugFlag = getEnvWithDefault("MM_SUP_DEBUG", debugMode).(bool)
	}
	debugMode = DebugFlag

	if !NoObfuscateFlag {
		NoObfuscateFlag = getEnvWithDefault("MM_SUP_NO_OBFUSCATE", false).(bool)
	}
	EnableObfuscation := !NoObfuscateFlag

	// Validate that Mattermost is present at either the default location, or the overridden location
	var ConfigFilePath string = MattermostDir + "/config/config.json"

	if !fileExists(ConfigFilePath) {
		LogMessage(warningLevel, "Config file not found at: "+ConfigFilePath)
		LogMessage(infoLevel, "Attempting default configuration for config file")
		ConfigFilePath = defaultMattermostDir + "/config/config.json"
		if !fileExists(ConfigFilePath) {
			LogMessage(errorLevel, "Unable to locate config file!")
			os.Exit(3)
		}
	}

	DebugPrint("MattermostDir: " + MattermostDir)
	DebugPrint("TargetDir: " + TargetDir)
	DebugPrint("PkgNamePrefix: " + PkgNamePrefix)

	// Log obfuscation status
	if EnableObfuscation {
		LogMessage(infoLevel, "Data obfuscation is ENABLED (use -no-obfuscate to disable)")
	} else {
		LogMessage(warningLevel, "Data obfuscation is DISABLED - sensitive data will NOT be masked!")
	}

	// Process config.json
	LogMessage(infoLevel, "Analysing config file...")
	CurrentConfig := new(mmConfig)

	CurrentConfig.ProcessConfigFile(ConfigFilePath, MattermostDir)

	// Is the log file directory overridden via the ENVIRONMENT? [see https://docs.mattermost.com/configure/environment-configuration-settings.html#logging]
	CurrentConfig.LogDirectory = getEnvWithDefault("MM_LOGSETTINGS_FILELOCATION", CurrentConfig.LogDirectory).(string)

	// Create a temp directory to hold the support packet.
	tempDirectory, err := createTempDir(TargetDir, PkgNamePrefix)
	if err != nil {
		LogMessage(errorLevel, "Unable to proceed without temp directory!  Error: "+err.Error())
		os.Exit(4)
	}
	LogMessage(infoLevel, "Creating support packet in: "+tempDirectory)

	// Copy all log files from the Mattermost directory to the temp directory
	LogMessage(infoLevel, "Copying Mattermost log files")
	err = CopyLogFiles(CurrentConfig.LogDirectory, tempDirectory)
	if err != nil {
		LogMessage(warningLevel, "Failed to copy Mattermost log files. Error: "+err.Error())
	}

	// Copy the config file to the temp directory
	LogMessage(infoLevel, "Copying Mattermost config file")
	err = CopyConfigFile(ConfigFilePath, tempDirectory)
	if err != nil {
		LogMessage(warningLevel, "Failed to copy the Mattermost config file. Error: "+err.Error())
	}

	// Gathering information from system services
	LogMessage(infoLevel, "Gathering service level information")
	if !GatherServiceMessages(tempDirectory) {
		LogMessage(warningLevel, "Not all service information was gathered")
	}

	// Gather details of top running processes
	LogMessage(infoLevel, "Gathering details of running processes")
	err = GetTopProcesses(tempDirectory)
	if err != nil {
		LogMessage(warningLevel, "Failed to get top processes. Error: "+err.Error())
	}

	// Get port listening info from netstat/ss
	LogMessage(infoLevel, "Checking port listening status")
	err = CheckListeningPort(CurrentConfig.ListenPort, tempDirectory)
	if err != nil {
		LogMessage(warningLevel, "Failed to locate listening port information.  Error: "+err.Error())
	}

	// Copy OS information files to target directory
	LogMessage(infoLevel, "Copying key OS information files")
	if !CopyOSInfoFiles(tempDirectory) {
		LogMessage(warningLevel, "Some OS info files may be missing!")
	}

	// Get the disk free space
	LogMessage(infoLevel, "Retrieving disk space information")
	err = GetDiskSpace(tempDirectory)
	if err != nil {
		LogMessage(warningLevel, "Failed to retrieve disk space utilisation")
	}

	// Obfuscate sensitive data in all collected files
	if EnableObfuscation {
		LogMessage(infoLevel, "Obfuscating sensitive data in logs, config, and system files")
		if err := ObfuscateDirectory(tempDirectory, "*"); err != nil {
			LogMessage(warningLevel, "Failed to obfuscate sensitive data. Error: "+err.Error())
		} else {
			LogMessage(infoLevel, "Obfuscation completed successfully")
		}
	}

	// Compress temp folder, in preparation for sending to Mattermost
	LogMessage(infoLevel, "Compressing suport packet")
	supportPacketName, err := CompressSupportPacket(tempDirectory, TargetDir)
	if err != nil {
		LogMessage(errorLevel, "Failed to create support package!  Please check temp directory and compress manually.")
		os.Exit(5)
	}

	LogMessage(infoLevel, "Support packet creation complete!  Please send the following file to Mattermost Support: "+supportPacketName)

}
