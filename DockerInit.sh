#!/bin/sh
case $1 in
    amd64)
        ARCH="64"
        FNAME="amd64"
        MTG_ARCH="amd64"
        ;;
    i386)
        ARCH="32"
        FNAME="i386"
        MTG_ARCH="386"
        ;;
    armv8 | arm64 | aarch64)
        ARCH="arm64-v8a"
        FNAME="arm64"
        MTG_ARCH="arm64"
        ;;
    armv7 | arm | arm32)
        ARCH="arm32-v7a"
        FNAME="arm32"
        MTG_ARCH="armv7"
        ;;
    armv6)
        ARCH="arm32-v6"
        FNAME="armv6"
        MTG_ARCH="armv6"
        ;;
    *)
        ARCH="64"
        FNAME="amd64"
        MTG_ARCH="amd64"
        ;;
esac
MTG_VER="2.2.8"
XRAY_VERSION="${XUI_XRAY_VERSION:-v26.6.1}"
case "$XRAY_VERSION" in
    v[0-9]*.[0-9]*.[0-9]*) ;;
    *)
        echo "Ignoring invalid XUI_XRAY_VERSION; using v26.6.1"
        XRAY_VERSION="v26.6.1"
        ;;
esac
XRAY_ASSET_URL_TEMPLATE="${XUI_XRAY_ASSET_URL_TEMPLATE:-https://github.com/XTLS/Xray-core/releases/download/{tag}/Xray-{os}-{arch}.zip}"
case "$XRAY_ASSET_URL_TEMPLATE" in
    *"{tag}"*"{os}"*"{arch}"*) ;;
    *)
        echo "Ignoring invalid XUI_XRAY_ASSET_URL_TEMPLATE; using default Xray release template"
        XRAY_ASSET_URL_TEMPLATE="https://github.com/XTLS/Xray-core/releases/download/{tag}/Xray-{os}-{arch}.zip"
        ;;
esac
xray_asset_url() {
    printf '%s' "$XRAY_ASSET_URL_TEMPLATE" \
        | sed "s|{tag}|$XRAY_VERSION|g; s|{os}|linux|g; s|{arch}|$ARCH|g"
}
curl_with_auth() {
    for CURL_URL do :; done
    if [ -n "${XUI_DOWNLOAD_AUTH_HEADER:-}" ] && [ "${CURL_URL#https://github.com/XTLS/Xray-core/releases/download/}" = "$CURL_URL" ]; then
        curl -H "$XUI_DOWNLOAD_AUTH_HEADER" "$@"
    else
        curl "$@"
    fi
}
mkdir -p build/bin
cd build/bin
curl_with_auth -sfLRo "Xray-linux-${ARCH}.zip" "$(xray_asset_url)"
unzip "Xray-linux-${ARCH}.zip"
rm -f "Xray-linux-${ARCH}.zip" geoip.dat geosite.dat
mv xray "xray-linux-${FNAME}"
curl -sfLRO "https://github.com/9seconds/mtg/releases/download/v${MTG_VER}/mtg-${MTG_VER}-linux-${MTG_ARCH}.tar.gz"
tar -xzf "mtg-${MTG_VER}-linux-${MTG_ARCH}.tar.gz"
mv "mtg-${MTG_VER}-linux-${MTG_ARCH}/mtg" "mtg-linux-${FNAME}" 2>/dev/null || mv mtg "mtg-linux-${FNAME}"
rm -rf "mtg-${MTG_VER}-linux-${MTG_ARCH}" "mtg-${MTG_VER}-linux-${MTG_ARCH}.tar.gz"
chmod +x "mtg-linux-${FNAME}"
curl -sfLRO https://github.com/Loyalsoldier/v2ray-rules-dat/releases/latest/download/geoip.dat
curl -sfLRO https://github.com/Loyalsoldier/v2ray-rules-dat/releases/latest/download/geosite.dat
curl -sfLRo geoip_IR.dat https://github.com/chocolate4u/Iran-v2ray-rules/releases/latest/download/geoip.dat
curl -sfLRo geosite_IR.dat https://github.com/chocolate4u/Iran-v2ray-rules/releases/latest/download/geosite.dat
curl -sfLRo geoip_RU.dat https://github.com/runetfreedom/russia-v2ray-rules-dat/releases/latest/download/geoip.dat
curl -sfLRo geosite_RU.dat https://github.com/runetfreedom/russia-v2ray-rules-dat/releases/latest/download/geosite.dat
cd ../../
