## apictl gen deployment-dir

Generate a sample deployment directory

### Synopsis

Generate a sample deployment directory based on the provided source artifact

```
apictl gen deployment-dir [flags]
```

### Examples

```
apictl gen deployment-dir -s  ~/PizzaShackAPI_1.0.0.zip
apictl gen deployment-dir -s  ~/PizzaShackAPI_1.0.0.zip  -d /home/Deployment_repo/Dev
```

### Options

```
  -d, --destination string   Path of the directory where the directory should be generated
  -h, --help                 help for deployment-dir
  -s, --source string        Path of the source directory to be used when generating the directory
```

### Options inherited from parent commands

```
  -k, --insecure   Allow connections to SSL endpoints without certs
      --verbose    Enable verbose mode
```

### SEE ALSO

* [apictl gen](apictl_gen.md)	 - Generate deployment directory for VM and K8S operator

