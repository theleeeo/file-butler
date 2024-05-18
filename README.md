# file-butler
A multi-source file proxy with custom auth

## Providers

## Auth-plugins

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

Deleting files is currently not supported.

### Presigned URLs

Presigned URLs can be generated for a file in a supported provider by using the `presign` request.
Refer to the provider documentation for information about weather or not this is supported by that provider.

Example: `GET /presign/s3provider/myfile.txt`

This will return a signed URL that can be used to download the file directly from the service behind the provider without going through the file-butler proxy.

### Experimental Features
Please note that the functionality described in this section is experimental and subject to change. Use it at your own risk.

#### Tags

Tags is key:value pairs that can be attached to files. Tags carry no meaning to the file-butler service, but can be used to store metadata about the file in the provider.

Tags must be set as query parameters in the request URL when uploading a file. The format is `?key=value&key=value`.
Example: `PUT /file/s3provider/myfile.txt?tag1=value1&tag2=value2`

This is not supported for all providers and for some providers, not including the tags when updating an existing files will cause the tags to be removed.
Please refer to the provider documentation for more information.