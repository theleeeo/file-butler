# file-butler

A multi-source file proxy with custom auth
This is still a work in progress. More features will be added and breaking changes may occur.

## Providers

Providers are the sources of files that the file-butler service can interact with.
Providers are defined in a separate configuration file and can be of different types.
The default provider configuration file is `providers.<ext>`. All extensions by the [Viper](https://github.com/spf13/viper) library can be used (JSON, TOML, YAML, HCL).
All examples in this document will use the TOML format.

The provider configuration file is watched by the file-butler service and will be reloaded live when the file is changed.
Triggering a file event (eg: saving the file in an editor or using the `touch` command) on the file will cause the service to reload all the providers, removing any that are no longer present and adding any new ones.

What type a configured provider will be is determined by the `type` field in the provider configuration.

The identifier of a provider is the key in the TOML file. This is used to reference the provider in requests to the file-butler service.

On all providers you can specify an `auth-plugin`. If present it will override the global defualt auth-plugin for that provider.

### AWS-S3

The AWS-S3 provider allows the user to interact with an S3 bucket. Its provider type is `s3`.

Provider config example:

```toml
[s3test]
type = "s3"
region = "eu-north-1"
bucket = "my-cool-bucket"
profile = "default"
presign-enabled = true
```

If presign is enabled, the provider will support the `presign` request type.
Default is `false`.

If a profile is specified, the provider will use the credentials from the specified profile in the AWS credentials file.
If no profile is specified, the provider will use the default credentials.

### Log

### Void

### CDK

## Auth-plugins

### Address

### Command

### Built-in

The built-in auth plugins are baked into the file-butler service and are always available.
They does not require any plugins to be created by the user and have a substantially lower overhead than the other auth plugins.

#### allow-types:

The `allow-types` plugin allows the user to specify which request types are allowed. It does not do any other checks.

The available types are:

- `download` (GET /file/)

- `upload` (PUT / POST /file/)
<!-- - `presign` (GET /presign) -->
- `get_tags` (GET /tags/)
<!-- - `set_tags` (PUT /tags) -->
- `list` (GET /list/)

Config example:

```toml
[[auth-plugins]]
name = "allow-only-files"
builtin = "allow-types"
args = ["download", "upload"]
```

## Endpoints

All endpoints take the form of `/<request>/<provider>/<key>`.

- `request` is the type of request to make (e.g. `file` for upload and download of files)

### Files

Files (or any arbitrary data) can be retrieved from a provider by using the `file` request.
Example: `GET /file/s3provider/myfile.txt`
This will try to return the file `myfile.txt` from the `s3provider`.

Files can be uploaded to a provider by using the `file` request with a `PUT` or `POST` method..
Example: `PUT /file/s3provider/myfile.txt`
This will upload the file `myfile.txt` to the `s3provider`.

If a Content-Type header is present in the request, it will be passed on to the provider.
If the provider supports it, that will be persisted with the file and returned in the response.
If no Content-Type header is present, the server will try to determine the content type of the file.

Deleting files is currently not supported.

### Presigned URLs

Presigned URLs can be generated for a file in a supported provider by using the `presign` request.

It is required to specify the operation to presign as a query parameter.
The operations available are:

- `download` - Generate a URL that can be used to download the file directly from the provider.
- `upload` - Generate a URL that can be used to upload a file directly to the provider.

Refer to the provider documentation for information about weather or not this is supported by that provider.

Example: `GET /presign/s3provider/myfile.txt?op=download`

This will return a signed URL that can be used to download the file directly from the service behind the provider without going through the file-butler proxy.

### Experimental Features

Please note that the functionality described in this section is experimental and subject to change. Use it at your own risk.

#### Tags

Tags is key:value pairs that can be attached to files. Tags carry no meaning to the file-butler service, but can be used to store metadata about the file in the provider.

Tags must be set as query parameters in the request URL when uploading a file. The format is `?tag=key:value&tag=key:value&...`.
Example: `PUT /file/s3provider/myfile.txt?tag=author:john&tag=type:document`

This is not supported for all providers and for some providers, not including the tags when updating an existing files will cause the tags to be removed.
Please refer to the provider documentation for more information.

The tags can be retrieved by using the `meta` request.
Example: `GET /meta/s3provider/myfile.txt`

This will return a JSON object with the tags as key:values of the field `tags`.
Example response:

```json
{
  "tags": {
    "author": "john",
    "type": "document"
  }
}
```

#### List

The `list` request can be used to list all files in a provider.
Example: `GET /list/s3provider/`

This will return a JSON array with the keys of all files in the provider.
Example response:

```json
["file1.txt", "file2.txt", "folder/file3.txt"]
```
