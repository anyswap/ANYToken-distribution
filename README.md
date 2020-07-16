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

prepare the following:

1. install mongodb
2. start mongod service
3. start fusion node (--gcmode archive)
4. config rightly

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

## byliquid subcommand

```text
OPTIONS:
   --rewardToken value  reward token
   --rewards value      total rewards (uint wei)
   --start value        start height (start inclusive)
   --end value          end height (end exclusive)
   --exchange value     exchange address
   --accounts value     accounts file (line format: <address>), read from database if not specified
   --keystore value     keystore file path
   --password value     password file path
   --gasLimit value     gas limit in transaction, use default if not specified
   --gasPrice value     gas price in transaction, use default if not specified
   --nonce value        nonce in transaction, use default if not specified
   --output value       output file of result
   --dryrun             dry run (default: false)
   --help, -h           show help (default: false)
```

The meaning of options is same as `byvolume` subcommand. please see it at next section.

only `--accounts` is different, which is used to manually specify all account list of the corresponing exchange.

Example:

```shell
./distribute -v 6 -c config.toml byliquid -rewardToken 0xd05a60de2893ddc485cb8e9868be9a4abfa02a6b -rewards 100000000000000000000 -start 100 -end 200 -exchange 0x9fd692e4e681b62b6cb62d5afe397573cbd33d32 -keystore keystore.json -password password.txt -accounts accounts.txt -output result.txt -dryrun
```

We should use `global options` `-c` to specify config file where the `MongoDB`, `Gateway` etc. are reightly configed.

## byvolume subcommand

```text
OPTIONS:
   --rewardToken value  reward token
   --rewards value      total rewards (uint wei)
   --start value        start height (start inclusive)
   --end value          end height (end exclusive)
   --exchange value     exchange address
   --volumes value      volumes file (line format: <address> <volume>), read from database if not specified
   --keystore value     keystore file path
   --password value     password file path
   --gasLimit value     gas limit in transaction, use default if not specified
   --gasPrice value     gas price in transaction, use default if not specified
   --nonce value        nonce in transaction, use default if not specified
   --output value       output file of result
   --dryrun             dry run (default: false)
   --help, -h           show help (default: false)
```

Example:

```shell
./distribute -v 6 -c config.toml byvolume -rewardToken 0xd05a60de2893ddc485cb8e9868be9a4abfa02a6b -rewards 100000000000000000000 -start 100 -end 200 -exchange 0x9fd692e4e681b62b6cb62d5afe397573cbd33d32 -keystore keystore.json -password password.txt -volumes volumes.txt -output result.txt -dryrun
```


#### options usage

`--rewardToken value` reward token

>	specify send which token(ERC20) as reward, its the token's contract address.

`--rewards value    ` total rewards (uint wei)

>	specify the total rewards to send in specified block height range.

`--start value      ` start height (start inclusive)

>	specify start height of block range, start is inclusive.

`--end value        ` end height (end exclusive)

>	specify end height of block range, end is exclusive.

`--exchange value   ` exchange address

>	specify which exchange is statistics for the liquidity and volume.

`--volumes value    ` volumes file (line format: <address> <volume>), read from database if not specified

>	specify volumes file manually. read from database if not specified.

`--keystore value   ` keystore file path

>	specify keystore file to sign transaction to send rewards.

`--password value   ` password file path

>	specify password file to sign transaction to send rewards.

`--gasLimit value   ` gas limit in transaction

>	specify gas limit to build transaction to send rewards. get from RPC is not specified.

`--gasPrice value   ` gas price in transaction

>	specify gas price to build transaction to send rewards. get from RPC is not specified.

`--nonce value      ` nonce in transaction

>	specify account nonce to build transaction to send rewards. get from RPC is not specified.

`--output value     ` output file of result

>	specify output result file. with line format: `<address> <reswards> <txhash>` (zero hash mean tx is not sent)

`--dryrun           ` dry run (default: false)

>	specify whether send transaction to blockchain or just dry run.
