# sox_aaS
SoX commands as a service

### Currently supported features:

* spectrogram

## With Docker
```bash
docker run -p 3000:3000 simon987/sox_aas
```

## From source

Instal dependencies: `apt install sox libsox-fmt-all`

```bash
git clone https://github.com/simon987/sox_aaS
cd sox_aaS
go build

API_ADDR=0.0.0.0:3000 ./sox_aaS
```
