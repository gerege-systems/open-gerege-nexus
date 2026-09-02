# Android dedicated-device provisioning

Factory-reset kiosk төхөөрөмж дээр kiosk flavor суулгасны дараа:

```sh
adb shell dpm set-device-owner mn.gerege.nexus.kiosk/mn.gerege.nexus.GeregeDeviceAdminReceiver
```

App нь device-owner эрхийг шалгаж өөрийн package-ийг Lock Task allowlist-д
оруулна. Production fleet дээр энэ үйлдлийг Android Enterprise zero-touch/QR
provisioning болон EMM managed configuration-аар хийнэ.
