# ANYToken distribution

## Building

```shell
git clone https://github.com/anyswap/ANYToken-distribution.git
cd ANYToken-distribution
make all
```

After building, the following files will be generated in `./build/bin` directory:

```text
distribute
config-example.toml
```

## Config file

Copy `config-example.toml` to `config.toml` and modify it accordingly.

Please refer [config file example](https://github.com/anyswap/ANYToken-distribution/blob/master/params/config-example.toml)

## Run distribute

```shell
setsid ./build/bin/distribute --verbosity 6 --config build/bin/config.toml --log build/bin/logs/distribute.log >/dev/null 2>&1
```

## Command line

Show help info, run

```shell
./build/bin/distribute --help
```

The following are command line options:

```text
   --config value, -c value     config file, use toml format
   --syncfrom value             sync start height, 0 means read from database (default: 0)
   --syncto value               sync end height (excluding end), 0 means endless (default: 0)
   --overwrite                  overwrite exist items in database (default: false)
   --verbosity value, -v value  log verbosity (0:panic, 1:fatal, 2:error, 3:warn, 4:info, 5:debug, 6:trace) (default: 4)
   --log value                  log file, support rotate
   --rotate value               log rotation time (unit hour) (default: 24)
   --maxage value               log max age (unit hour) (default: 720)
   --json                       output log in json format (default: false)
   --color                      output log in color text format (default: true)
   --help, -h                   show help (default: false)
```
