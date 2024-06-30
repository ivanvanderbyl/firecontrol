# FireControl

FireControl implements a package and CLI for controlling Escea fireplaces in Australia and New Zealand.

It was largely implemented by reverse engineering the wire-protocol used by the Escea iOS app to communicate with the fireplace over the local network.


> [!CAUTION]
> This is not an official project endorsed by Escea. It is a reverse-engineered implementation of the wire protocol used by the Escea iOS app to communicate with the fireplace over the local network.
>
> It is controlling a fireplace, so use it at your own risk. If it burns your house down, it's not my fault.
>
> **Absolutely no warranty is provided.**

## Usage

```bash
firecontrol --help
```

## HomeKit integration

FireControl can be integrated with HomeKit using the `firecontrol homekit-accessory` command.

```bash
firecontrol homekit-accessory --help
```

You'll need to run this on a local server or Raspberry Pi that is always on and connected to the same network as the fireplace in order for it to remain available in HomeKit.
