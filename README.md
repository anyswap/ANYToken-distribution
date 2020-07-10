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
setsid ./build/bin/distribute --verbosity 6 --config build/bin/config.toml --log build/bin/logs/distribute.log
```
