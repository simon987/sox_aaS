package main

import (
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"html/template"
	"io/ioutil"
	"os"
	"os/exec"
	"strconv"
)

var logger, _ = zap.NewProduction()

func Spectrogram(file []byte, x, y, z int64, label string) ([]byte, error) {

	cmd := exec.Command(
		"sox",
		"-",
		"-n", "remix", "1", "spectrogram",
		"-t", label,
		"-x", strconv.FormatInt(x, 10), "-y", strconv.FormatInt(y, 10), "-z", strconv.FormatInt(z, 10),
		"-w", "Kaiser",
		"-o", "-",
	)

	in, _ := cmd.StdinPipe()

	out, _ := cmd.StdoutPipe()

	err := cmd.Start()
	if err != nil {
		logger.Error("Error while starting process:",
			zap.Error(err),
		)
		return nil, err
	}

	_, err = in.Write(file)
	_ = in.Close()

	if err != nil {
		logger.Error("Error during stdin write:",
			zap.Error(err),
		)
		return nil, err
	}

	data, _ := ioutil.ReadAll(out)
	_ = out.Close()

	err = cmd.Wait()
	if err != nil {
		logger.Error("Error during wait:",
			zap.Error(err),
		)
		return nil, err
	}

	logger.Info("executed command",
		zap.Int("exit code", cmd.ProcessState.ExitCode()),
	)

	return data, nil
}

type SpectrogramReq struct {
	Data  []byte `json:"data"`
	Label string `json:"label"`
	X     int64  `json:"x"`
	Y     int64  `json:"y"`
	Z     int64  `json:"z"`
}

func (req *SpectrogramReq) IsValid() bool {
	if req.X < 100 || req.X > 200000 {
		return false
	}

	if req.Y < 100 || req.Y > 10000 {
		return false
	}

	if req.Z < 20 || req.Z > 180 {
		return false
	}

	if len(req.Data) == 0 {
		return false
	}

	return true
}

func SpectrogramHandler(ctx *gin.Context) {

	var req SpectrogramReq

	_ = ctx.Request.ParseMultipartForm(256 << 20)
	dataFile, err := ctx.FormFile("data")

	if err != nil {
		ctx.JSON(400, map[string]string{
			"error": "Invalid request",
		})
		return
	}

	req.Data = make([]byte, dataFile.Size)
	file, _ := dataFile.Open()
	_, err = file.Read(req.Data)
	if err != nil {
		ctx.JSON(400, map[string]string{
			"error": "Invalid request",
		})
		return
	}

	req.X, _ = strconv.ParseInt(ctx.PostForm("x"), 10, 64)
	req.Y, _ = strconv.ParseInt(ctx.PostForm("y"), 10, 64)
	req.Z, _ = strconv.ParseInt(ctx.PostForm("z"), 10, 64)
	req.Label = ctx.PostForm("label")

	if !req.IsValid() {
		ctx.JSON(400, map[string]string{
			"error": "Invalid request",
		})
		return
	}

	out, err := Spectrogram(req.Data, req.X, req.Y, req.Z, req.Label)
	if err != nil {
		ctx.JSON(500, map[string]string{
			"error": err.Error(),
		})
		return
	}

	ctx.Data(200, "image/png", out)
}

type indexData struct {
}

func IndexHandler(ctx *gin.Context) {

	ctx.Header("Content-Type", "text/html")

	var data indexData

	t, _ := template.New("index").Parse(
		`
<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <title>sox_aaS</title>
</head>
<body>

<form method="post" id="form" enctype="multipart/form-data" action="/api/spectrogram">
    <input type="number" name="x" id="x" placeholder="x" value="3000">
    <input type="number" name="y" id="y" placeholder="y" value="500">
    <input type="number" name="z" id="z" placeholder="y" value="120">
    <input type="text" name="label" id="label" placeholder="label" value="Hello, world">
	<input type="file" name="data" id="upload" style="display: none">
</form>

<button onclick="onUpload()">Spectrogram</button>

</body>

<script>

function onUpload() {
    const uploadElem = document.getElementById("upload");
    uploadElem.click()

    uploadElem.onchange = () => {
		document.getElementById("form").submit();
    }
}
</script>
</html>`,
	)
	t.Execute(ctx.Writer, data)
}

func main() {
	gin.SetMode(gin.ReleaseMode)
	defer logger.Sync()

	r := gin.Default()
	r.POST("/api/spectrogram", SpectrogramHandler)
	r.GET("/", IndexHandler)

	addr := os.Getenv("API_ADDR")
	if addr == "" {
		addr = "localhost:3000"
	}

	r.Run(addr)
}
