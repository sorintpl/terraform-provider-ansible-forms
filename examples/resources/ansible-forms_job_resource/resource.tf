resource "ansible-forms_job_resource" "job" {
  cx_profile_name = "cluster1"
  form_name       = "Demo Form Ansible No input"
  extravars = {
    region               = "myregion"
    opco                 = "myopco"
    svm_name             = "mysvm_name"
    exposure             = "myexposure"
    env                  = "myenv"
    dataclass            = "mydataclass"
    share_name           = "myshare_name"
    accountid            = "myaccountid"
    size                 = "mysize"
    protection_required  = "myprotection_required"
    some_custom_property = "property"
  }
  credentials = {
    ontap_cred = "myontap_cred"
    cifs_cred  = "mycifs_cred"
  }
#   state = "absent"
}

output "ansible-forms_job_resource" {
  value = ansible-forms_job_resource.job
}
