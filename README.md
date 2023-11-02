# mm-packet-pull
This package is used to pull a collection of useful support information for troubleshooting a Mattermost instance that is refusing to start.
It prepares a tar file containing key information from both Mattermost and the underlying OS that can then be quickly sent to Mattermost Support, without the user
having to manually collect this key information.

## Usage
```
Usage of mm-packet-pull_<os-version>:
  -debug
    	Enable debug mode.
  -directory string
    	Install directory of Mattermost. [Default: /opt/mattermost]
  -name string
    	Prefix for name of support packet. [Default: support-packet]
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
| `--debug` | `MM_SUP_DEBUG` | Enables debug output |

## Deployment

Rather than having to install a Go development environment, this project is available as a binary for AMD64 Linux environments, at [this link](https://github.com/jlandells/mm-packet-pull/releases/download/v0.1.0/mm-packet-pull_linux_amd64)