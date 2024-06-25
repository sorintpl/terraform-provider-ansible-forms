# Terraform AnsibleForms Provider

Custom AnsibleForms Provider that allows Terraform to run Ansible playbooks based on data sent using AnsibleForms.

## Requirements

- [Terraform](https://developer.hashicorp.com/terraform/downloads) >= 1.4
- [Go](https://golang.org/doc/install) >= 1.21

## Building The Provider

1. Clone the repository
1. Enter the repository directory
1. Build the provider using the Go `install` command:

```shell
go install
```

## Notes

Provider checks AnsibleForms job's status every **15 seconds** and will abort Terraform work after **3 minutes** if there is no result (both time periods can be configured in code by changing values of `CheckLoopInterval` and `CheckLoopTimeout` constants in ***internal/restclient/rest_client.go*** file).