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
        versionName = "1.0.0"
    }
    buildFeatures { compose = true; buildConfig = true }
    compileOptions { sourceCompatibility = JavaVersion.VERSION_17; targetCompatibility = JavaVersion.VERSION_17 }
    kotlinOptions { jvmTarget = "17" }
    // Гарын шугам нэг л build — ширээний олон form factor энд байхгүй
    // (`shared/device_lines.json`: mobile нь iOS ба Android хоёрын нийтлэг шугам).
}

// Гэрээний тест нь AndroidManifest-ийг ФАЙЛААР уншдаг тул Gradle-д түүнийг
// оролт гэж хэлнэ — эс бөгөөс манифест өөрчлөгдөхөд тест «up-to-date» гэж
// алгасагдаж, зөрүүг барих ёстой шалгалт чимээгүй хоцордог.
tasks.withType<Test>().configureEach {
    inputs.file("src/main/AndroidManifest.xml")
}

dependencies {
    implementation("com.squareup.okhttp3:okhttp:4.12.0")
    implementation(platform("androidx.compose:compose-bom:2026.06.00"))
    implementation("androidx.activity:activity-compose:1.12.3")
    implementation("androidx.compose.material3:material3")
    implementation("androidx.compose.material:material-icons-extended")
    implementation("androidx.compose.ui:ui")
    implementation("androidx.compose.ui:ui-tooling-preview")
    implementation("androidx.lifecycle:lifecycle-runtime-compose:2.9.4")
    debugImplementation("androidx.compose.ui:ui-tooling")
    testImplementation("junit:junit:4.13.2")
}
