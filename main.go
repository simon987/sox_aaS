package main

import (
	"github.com/ReneKroon/ttlcache"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.uber.org/zap"
	"html/template"
	"io/ioutil"
	"os"
	"os/exec"
	"strconv"
	"time"
)

var logger, _ = zap.NewProduction()
var cache = ttlcache.NewCache()

func Spectrogram(file []byte, x, y, z int64, label, window string) ([]byte, error) {

	cmd := exec.Command(
		"sox",
		"-",
		"-n", "remix", "1", "spectrogram",
		"-t", label,
		"-x", strconv.FormatInt(x, 10), "-y", strconv.FormatInt(y, 10), "-z", strconv.FormatInt(z, 10),
		"-w", window,
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
	Data   []byte `json:"data"`
	Label  string `json:"label"`
	X      int64  `json:"x"`
	Y      int64  `json:"y"`
	Z      int64  `json:"z"`
	Window string `json:"window"`
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

	if req.Window != "Hann" && req.Window != "Hamming" && req.Window != "Bartlett" &&
		req.Window != "Rectangular" && req.Window != "Kaiser" && req.Window != "Dolph" {
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
	req.Window = ctx.PostForm("window")

	if !req.IsValid() {
		ctx.JSON(400, map[string]string{
			"error": "Invalid request",
		})
		return
	}

	out, err := Spectrogram(req.Data, req.X, req.Y, req.Z, req.Label, req.Window)
	if err != nil {
		ctx.JSON(500, map[string]string{
			"error": err.Error(),
		})
		return
	}

	key := uuid.New().String()
	cache.Set(key, out)

	ctx.Redirect(302, "/api/image/"+key)
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
	<style>
	body {
		font-family: "Helvetica Neue",Helvetica,Arial,sans-serif;
	}
	label {
		display: block;
		margin-bottom: 0.5em;
	}
	.box {
		background: #90CAF9;
		margin-left: auto;
		margin-right: auto;
		width: 500px;
		padding: 1em;
		box-shadow: 0 3px 6px rgba(0,0,0,0.16), 0 3px 6px rgba(0,0,0,0.23);
		margin-top: 1em;	
	}
	button {
		margin-left: auto;
		display: block;
	}
	h3 {
		text-align: center;
	}
	</style>
</head>
<body>

<div class="box">
	<form method="post" id="form" enctype="multipart/form-data" action="/api/spectrogram">
		<h3>Spectrogram</h3>
		<label for="x">
			X
			<input type="number" name="x" id="x" placeholder="x" value="3000" min="100" max="200000">
		</label>
		<label for="y">
			Y
			<input type="number" name="y" id="y" placeholder="y" value="500" min="100" max="200000">
		</label>
		<label for="z">
			Z
			<input type="number" name="z" id="z" placeholder="z" value="100" min="20" max="120">
		</label>
		<label for="z">
			Label
			<input type="text" name="label" id="label" placeholder="label">
		</label>
		<label for="window">
			Window
			<select name="window" id="window">
				<option>Hann</option>
				<option>Hamming</option>
				<option>Bartlett</option>
				<option>Rectangular</option>
				<option selected>Kaiser</option>
				<option>Dolph</option>
			</select>
		</label>
		<input type="file" name="data" id="upload" style="display: none">
	</form>

	<button onclick="onUpload()">Upload</button>
</div>


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

func ImageHandler(ctx *gin.Context) {
	value, ok := cache.Get(ctx.Param("key"))
	if !ok {
		ctx.AbortWithStatus(404)
		return
	}
	ctx.Data(200, "image/png", value.([]byte))
}

func main() {
	gin.SetMode(gin.ReleaseMode)
	defer logger.Sync()
	defer cache.Close()
	cache.SetTTL(5 * time.Minute)

	r := gin.Default()
	r.POST("/api/spectrogram", SpectrogramHandler)
	r.GET("/", IndexHandler)
	r.GET("/api/image/:key", ImageHandler)

	addr := os.Getenv("API_ADDR")
	if addr == "" {
		addr = "localhost:3000"
	}

	r.Run(addr)
}
