# mm-packet-pull

![version](https://img.shields.io/github/v/tag/jlandells/mm-packet-pull?label=version)

This package is used to pull a collection of useful support information for troubleshooting a Mattermost instance that is refusing to start.
It prepares a tar.gz file containing key information from both Mattermost and the underlying OS that can then be quickly sent to Mattermost Support, without the user
having to manually collect this key information.

**By default, sensitive data in logs and configuration files is automatically obfuscated to protect privacy while maintaining troubleshooting capabilities.**

## Usage
```
Usage of mm-packet-pull_<os-version>:
  -debug
    	Enable debug mode.
  -directory string
    	Install directory of Mattermost. [Default: /opt/mattermost]
  -name string
    	Prefix for name of support packet. [Default: support-packet]
  -no-obfuscate
    	Disable obfuscation of sensitive data in logs and config files. [Default: obfuscation enabled]
  -target string
    	Target directory in which the support packet will be created. [Default: /tmp]
```

**Note**: This utility needs to be run with `sudo`, and will fail if run as a regular user.  This is due to the need to copy files from the `mattermost` user, as well as reading some system files.

### Parameters
All command line parameters can have equivalent environment variables, in order to simplify repeated executions.  Note that the command line will overrider the environment variables.

| Command Line | Environment Var | Description |
|--------------|-----------------|-------------|
| `--directory <dir>` | `MM_SUP_DIR` | Path to Mattermost directory, if not the default of `/opt/mattermost` |
| `--target <dir>` | `MM_SUP_TGT` | Path to a specific directory for the package.  Default is `/tmp` |
| `--name <name>` | `MM_SUP_NAME` | Name of the customer or other name to use for the prefix of the package filename |
| `--no-obfuscate` | `MM_SUP_NO_OBFUSCATE` | Disables obfuscation of sensitive data (passwords, IPs, emails, etc.) |
| `--debug` | `MM_SUP_DEBUG` | Enables debug output |

## Data Obfuscation

By default, `mm-packet-pull` automatically obfuscates sensitive data in configuration files, log files, and system information to protect privacy while maintaining the ability to troubleshoot issues effectively.

### What Gets Obfuscated

The following types of sensitive data are automatically masked:

#### In Configuration Files (config.json)
- **Passwords**: All password fields → `***REDACTED***`
- **API Keys & Tokens**: API keys, secrets, tokens → `OBFUSCATED_KEY_xxxxxxxx` (consistent hash)
- **Database Credentials**: Database connection strings with masked usernames, passwords, hosts, and database names
  - Example: `postgres://user:pass@10.0.1.5:5432/mattermost` → `postgres://user_abc123:***REDACTED***@XXX.XXX.XXX.def/db_ghi789`
- **Email Addresses**: `user@example.com` → `user_abc123@domain_def456.com`
- **URLs**: Full URLs with obfuscated hostnames while preserving paths
- **IP Addresses**: `192.168.1.100` → `XXX.XXX.XXX.abc`
- **Usernames**: Replaced with consistent hash-based values
- **Encryption Salts**: Masked like API keys

#### In Log Files
- **IP Addresses**: All IPv4 addresses → `XXX.XXX.XXX.xxx` (last segment from consistent hash)
- **Email Addresses**: Obfuscated with consistent hashing
- **URLs**: Hostnames and domains masked, paths preserved
- **Long Tokens**: Strings 40+ characters → `OBFUSCATED_KEY_xxxxxxxx`
- **User IDs**: Mattermost 26-character IDs → `id_xxxxxxxx`

### Obfuscation Consistency

The obfuscation uses **consistent hashing**, meaning:
- The same value will always map to the same obfuscated value within a single run
- Support engineers can still track the same IP, email, or user across different log entries
- Different actual values will map to different obfuscated values

**Example**: If IP `192.168.1.100` appears 50 times in your logs, it will be consistently obfuscated to the same value (e.g., `XXX.XXX.XXX.a1b`) throughout all files, making it possible to track connection patterns.

### What Is NOT Obfuscated

The following information is preserved for troubleshooting:
- Port numbers
- URL paths and query parameters (only hostnames are obfuscated)
- Timestamps
- Log levels and error messages
- System resource information (CPU, memory, disk usage)
- Process information
- Configuration structure and non-sensitive settings

### Disabling Obfuscation

If you need to include unobfuscated data (e.g., for internal troubleshooting), use the `--no-obfuscate` flag:

```bash
sudo ./mm-packet-pull --no-obfuscate
```

**Warning**: When obfuscation is disabled, all sensitive data (passwords, IP addresses, emails, tokens, etc.) will be included in plain text in the support packet.

### Reviewing the Support Packet

When reviewing the generated `.tar.gz` file, you should expect to see:

- **Config files**: JSON structure intact, but sensitive values replaced with placeholder text or consistent hashes
- **Log files**: Readable logs with IP addresses, emails, and tokens masked but patterns preserved
- **System files**: OS information with IP addresses obfuscated

The obfuscation is designed to allow Mattermost Support to effectively troubleshoot issues while protecting your organization's sensitive information.

## Deployment

Rather than having to install a Go development environment, this project is available as a binary for AMD64 Linux environments, at [this link](https://github.com/jlandells/mm-packet-pull/releases/download/v0.1.0/mm-packet-pull_linux_amd64)