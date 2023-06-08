# Fog

Fog is a CLI tool for generating small local clouds of virtual machines.

Fog uses QEMU under the hood to create and manage VMs and provisions instances with Cloud-init.

The UX is similar to Docker Compose but using full VMs instead of containers.

## Highlights

- Declarative, per project configuration files
- First class support for running clusters of VMs
- Uses official cloud images and Cloud-init for provisioning
- Near native performance on Linux and MacOS
- Automatic image management with fast and space efficient container style overlay filesystems

## Why?

Sometimes you can't use containers and you need a full VM. Vagrant is great but it predates both QEMU and Cloud-init and hasn't evolved to fully take advantage of them. Building something from scratch around these technologies has a lot of benefits.

QEMU is really fast, straightforward to install, and incredibly powerful. QEMU is able to use Apple’s Hypervisor framework (HVF) or Linux's KVM (Kernel-based Virtual Machine) for near native performance.

Using cloud images helps with production parity, vastly improves startup speeds over regular install images, and means you don't need to rely on the provider creating special "boxes". Most distros already publish QEMU compatible cloud images.

Cloud-init makes provisioning straightforward, and cloud-config is portable across every major cloud provider.

## Basic Usage

Fog is configured via a `fog.yaml` file in the project's root directory. The file specifies one or more `machines` that should be started. For example, the following definition creates a single machine named `lunar` which enables SSH on port `2222` and installs `jq`:

```yaml
machines:
  lunar:
    image: ubuntu:lunar
    ports:
      - "tcp::2222-:22"
    cloud_config:
      password: password
      chpasswd:
        expire: False
      ssh_pwauth: True
      packages:
        - jq
```

To boot the machines, just run `fog up`. You should see something like the following:

```shell-session
$ fog up
Using project config file: /home/matt/dev/fog/fog.yaml
Trying to pull ubuntu 8c1d7df2...
Already exists
2023/06/08 13:09:39 Running IMDS server on port 35049
2023/06/08 13:09:39 Using socket: /run/user/1000/fog/14b041740d916b2a45acf88cea5f6a16c1d840f894974dc19440bcc5dec7b50c.sock
2023/06/08 13:09:39 Using monitor socket: /run/user/1000/fog/14b041740d916b2a45acf88cea5f6a16c1d840f894974dc19440bcc5dec7b50c_monitor.sock
Starting lunar...
lunar    │ [    0.000000] Linux version 6.2.0-20-generic (buildd@lcy02-amd64-035) (x86_64-linux-gnu-gcc-12 (Ubuntu 12.2.0-17ubuntu1) 12.2.0, GNU ld (GNU Binutils for Ubuntu) 2.40) #20-Ubuntu SMP PREEMPT_DYNAMIC Thu Apr  6 07:48:48 UTC 2023 (Ubuntu 6.2.0-20.20-generic 6.2.6)
lunar    │ [    0.000000] Command line: BOOT_IMAGE=/boot/vmlinuz-6.2.0-20-generic root=LABEL=cloudimg-rootfs ro console=tty1 console=ttyS0
lunar    │ [    0.000000] KERNEL supported cpus:
# Lots more output...
```

Eventually you should see a line saying that Cloud-init finished, like this:

```
lunar    │ [   17.353892] cloud-init[792]: Cloud-init v. 23.1.2-0ubuntu0~23.04.1 finished at Thu, 08 Jun 2023 17:10:11 +0000. Datasource DataSourceNoCloudNet [seed=dmi,http://10.0.2.2:35049/14b041740d916b2a45acf88cea5f6a16c1d840f894974dc19440bcc5dec7b50c/][dsmode=net].  Up 17.34 seconds
```

Once Cloud-init has finished any services you booted (such as SSH) should be available on the bound ports. In another terminal try SSHing into the instance.

When you are done with the machines, just press `Ctrl+C` to send a SIGINT and kill the VMs.

## Current Status

Fog is still a work in progress. It's usable for testing cloud configs but that's about it. It probably doesn't work correctly on MacOS or Windows yet. Only a few VM images are available and it's not possible to extend them yet.
