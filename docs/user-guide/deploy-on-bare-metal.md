# Openshift deployment with OAS - On Bare Metal
This guide contains all the sections regarding Bare Metal deployment method, like iPXE/PXE, VirtualMedia, etc... let's get started

## General

This section is generic for the most of the cases:

- DHCP/DNS running on the network you wanna deploy the OCP cluster.
- Assisted Installer up & running (It's ok if you're working with cloud version).
- Typical DNS entries for API VIP and Ingress VIP.
- Pull Secret to reach the OCP Container Images.
- SSH Key pair.

_*Note*: This method could be used also in Virtual environment_

- With that we could start, first step is create the cluster
- Fill the Cluster name and Pull Secret fields, also select the version you wanna deploy:

![img](img/new_cluster.png)

- Now fill the Base Domain field and the SSH Host Public Key

![img](img/entry_base_domain.png)
![img](img/entry_ssh_pub_key.png)

- Click on _Download Discovery ISO_

![img](img/entry_ssh_pub_key.png)

- Fill again the SSH public key and click on _Generate Discovery ISO_

![img](img/entry_ssh_download_discovery.png)

- Wait for ISO generation to finish and you will reach this checkpoint

![img](img/discovery_iso_generated.png)


## iPXE

*NOTE*: We use a sample URL, please change to fit your use case accordingly

### Accessing the iPXE boot script and artifacts

The service serves an iPXE boot script for each infra-env
The script can be downloaded using:
```
GET /api/assisted-install/v2/infra-envs/{infra_env_id}/downloads/files?file_name=ipxe-script
```
This URL can either be used directly or the script can be downloaded and hosted separately.

```
#!ipxe
initrd --name initrd http://assisted.example.com:8888/images/a7acfb01-d89f-40c8-82d7-02b20cf00173/pxe-initrd?arch=x86_64&version=4.9
kernel http://assisted.example.com:8888/boot-artifacts/kernel?arch=x86_64&version=4.9 initrd=initrd coreos.live.rootfs_url=http://assisted.example.com:8888/boot-artifacts/rootfs?arch=x86_64&version=4.9 random.trust_cpu=on rd.luks.options=discard ignition.firstboot ignition.platform.id=metal console=tty1 console=ttyS1,115200n8 coreos.inst.persistent-kargs="console=tty1 console=ttyS1,115200n8"
boot
```

A presigned URL for the script URL can be retrieved using:
`GET /api/assisted-install/v2/infra-envs/{infra_env_id}/downloads/files-presigned?file_name=ipxe-script`

### Booting the nodes from iPXE

- First step, we need to set up the boot mode on the iDrac's as `boot once` for iPXE, this will depend on the steps on every Bare Metal Manufacturer/Version/Hardware.
- When you are booting the nodes, stay tuned to press `crtl-b` when the prompt say that:

![img](img/iPXE_boot.png)

- Now we need to get a correct IP and point to the right iPXE file url from above
- And we just need to wait until the boot was finished, and the nodes start appearing on the Assisted Service interface

![img](img/manual_ipxe_boot.png)

![img](img/boot_from_ipxe.gif)

- Then we will modify the nodename to use a right name for Openshift

![img](img/ai_node_appear.gif)

- Create another 2 more nodes and repeat this step

![img](img/ai_all_nodes.png)

- Now fill the _API Virtual IP_ and _Ingress Virtual IP_ fields

![img](img/ai_vips.png)

- Now you just need to click on _Install Cluster_ button and wait for the installation to finish.
