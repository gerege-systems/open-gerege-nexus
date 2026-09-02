plugins {
    id("com.android.application")
    kotlin("android")
    id("org.jetbrains.kotlin.plugin.compose")
}

android {
    namespace = "mn.gerege.nexus"
    compileSdk = 36

    defaultConfig {
        applicationId = "mn.gerege.nexus"
        minSdk = 26
        targetSdk = 35
        versionCode = 1
        versionName = "0.1.0"
    }

    flavorDimensions += "formFactor"
    productFlavors {
        create("mobile") { dimension = "formFactor"; buildConfigField("String", "FORM_FACTOR", "\"mobile\"") }
        create("tablet") { dimension = "formFactor"; applicationIdSuffix = ".tablet"; buildConfigField("String", "FORM_FACTOR", "\"tablet\"") }
        create("kiosk") { dimension = "formFactor"; applicationIdSuffix = ".kiosk"; buildConfigField("String", "FORM_FACTOR", "\"kiosk\"") }
        create("pos") { dimension = "formFactor"; applicationIdSuffix = ".pos"; buildConfigField("String", "FORM_FACTOR", "\"pos\"") }
    }
    buildFeatures { compose = true; buildConfig = true }
    compileOptions { sourceCompatibility = JavaVersion.VERSION_17; targetCompatibility = JavaVersion.VERSION_17 }
    kotlinOptions { jvmTarget = "17" }
}

dependencies {
    implementation(project(":core"))
    implementation("com.squareup.okhttp3:okhttp:4.12.0")
    implementation(platform("androidx.compose:compose-bom:2026.06.00"))
    implementation("androidx.activity:activity-compose:1.12.3")
    implementation("androidx.compose.material3:material3")
    implementation("androidx.compose.ui:ui")
    implementation("androidx.compose.ui:ui-tooling-preview")
    implementation("androidx.lifecycle:lifecycle-runtime-compose:2.9.4")
    implementation("androidx.biometric:biometric:1.1.0")
    implementation("androidx.webkit:webkit:1.14.0")
    implementation("org.jetbrains.kotlinx:kotlinx-coroutines-android:1.10.1")
    implementation("com.google.zxing:core:3.5.3")
    debugImplementation("androidx.compose.ui:ui-tooling")
}
