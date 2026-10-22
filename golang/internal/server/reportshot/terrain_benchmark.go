package reportshot

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"time"
)

// CaptureTerrainBenchmark renders the generated HTML with the installed browser,
// scores identical cameras and saves both canvas images and a UI screenshot.
func CaptureTerrainBenchmark(ctx context.Context, out, onlyRegion string) error {
	listener, e := net.Listen("tcp", "127.0.0.1:0")
	if e != nil {
		return e
	}
	srv := &http.Server{Handler: http.FileServer(http.Dir(filepath.Dir(out))), ReadHeaderTimeout: 10 * time.Second}
	defer srv.Close()
	go srv.Serve(listener)
	b, e := openBrowser(ctx)
	if e != nil {
		return e
	}
	defer b.close()
	if e = prepareViewport(ctx, b); e != nil {
		return e
	}
	if _, e = b.call(ctx, "Page.navigate", map[string]string{"url": "http://" + listener.Addr().String() + "/" + filepath.Base(out) + "/"}); e != nil {
		return e
	}
	readyDeadline := time.Now().Add(60 * time.Second)
	for {
		var ready bool
		var failure string
		if e = b.evaluate(ctx, "!!window.terrainReady", &ready); e != nil {
			return e
		}
		if ready {
			break
		}
		if e = b.evaluate(ctx, "window.terrainError||''", &failure); e != nil {
			return e
		}
		if failure != "" {
			return fmt.Errorf("terrain viewer: %s", failure)
		}
		if time.Now().After(readyDeadline) {
			return fmt.Errorf("terrain viewer did not become ready")
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(200 * time.Millisecond):
		}
	}
	frameDir := filepath.Join(out, "frames")
	if e = os.MkdirAll(frameDir, 0755); e != nil {
		return e
	}
	var regions []struct {
		Name   string `json:"name"`
		Stages []struct {
			Name string `json:"name"`
		} `json:"stages"`
	}
	data, e := os.ReadFile(filepath.Join(out, "report.json"))
	if e != nil {
		return e
	}
	var report struct {
		Regions json.RawMessage `json:"regions"`
	}
	if e = json.Unmarshal(data, &report); e != nil {
		return e
	}
	if e = json.Unmarshal(report.Regions, &regions); e != nil {
		return e
	}
	preferredStage := func(index int) string {
		for _, stage := range regions[index].Stages {
			if stage.Name == "export-48" {
				return stage.Name
			}
		}
		return regions[index].Stages[len(regions[index].Stages)-1].Name
	}
	firstRegion := 0
	if onlyRegion != "" {
		firstRegion = -1
		for i, region := range regions {
			if region.Name == onlyRegion {
				firstRegion = i
			}
		}
		if firstRegion < 0 {
			return fmt.Errorf("unknown capture region: %s", onlyRegion)
		}
	}
	records := []map[string]any{}
	checks := []map[string]any{}
	switches := []map[string]any{}
	comparisonRegions := map[string]bool{}
	for i, region := range regions {
		if onlyRegion != "" && i != firstRegion {
			continue
		}
		stage := preferredStage(i)
		stageJSON, _ := json.Marshal(stage)
		if e = evaluateTerrainPromise(ctx, b, fmt.Sprintf("window.terrainBenchmark.select(%d,%s)", i, stageJSON), nil); e != nil {
			return e
		}
		has48, has63 := false, false
		for _, candidate := range region.Stages {
			has48 = has48 || candidate.Name == "export-48"
			has63 = has63 || candidate.Name == "export-63"
		}
		if has48 && has63 {
			comparisonRegions[region.Name] = true
			var result json.RawMessage
			if e = evaluateTerrainPromise(ctx, b, terrainSwitchCheck, &result); e != nil {
				return e
			}
			switches = append(switches, map[string]any{"region": region.Name, "check": result})
			fmt.Println("CACHED BUDGET SWITCH CHECK", region.Name, string(result))
		}
		var check json.RawMessage
		if e = evaluateTerrainPromise(ctx, b, "window.terrainBenchmark.validateNative()", &check); e != nil {
			return e
		}
		checks = append(checks, map[string]any{"region": region.Name, "check": check})
		fmt.Println("NATIVE CPU/GPU CHECK", region.Name, string(check))
		for _, view := range []string{"top", "oblique", "lit", "close"} {
			fmt.Println("CAPTURE CAMERA", region.Name, view)
			stage := preferredStage(i)
			stageJSON, _ := json.Marshal(stage)
			viewJSON, _ := json.Marshal(view)
			var capture struct {
				Score json.RawMessage `json:"score"`
				Wow   string          `json:"wow"`
				WC3   string          `json:"wc3"`
			}
			script := fmt.Sprintf("window.terrainBenchmark.capture(%d,%s,%s)", i, stageJSON, viewJSON)
			if e = evaluateTerrainPromise(ctx, b, script, &capture); e != nil {
				return e
			}
			for name, image := range map[string]string{"wow": capture.Wow, "wc3": capture.WC3} {
				comma := strings.IndexByte(image, ',')
				if comma < 0 {
					return fmt.Errorf("missing canvas image")
				}
				png, e := base64.StdEncoding.DecodeString(image[comma+1:])
				if e != nil {
					return e
				}
				if e = os.WriteFile(filepath.Join(frameDir, region.Name+"-"+view+"-"+name+".png"), png, 0644); e != nil {
					return e
				}
			}
			records = append(records, map[string]any{"region": region.Name, "view": view, "stage": stage, "score": capture.Score})
		}
	}
	// Exercise the full-8K source path as well as the lighter region overview.
	stageJSON, _ := json.Marshal(preferredStage(firstRegion))
	if e = evaluateTerrainPromise(ctx, b, fmt.Sprintf("window.terrainBenchmark.select(%d,%s,'0')", firstRegion, stageJSON), nil); e != nil {
		return e
	}
	if comparisonRegions[regions[firstRegion].Name] {
		var result json.RawMessage
		if e = evaluateTerrainPromise(ctx, b, terrainSwitchCheck, &result); e != nil {
			return e
		}
		switches = append(switches, map[string]any{"region": regions[firstRegion].Name, "focus": "8K ADT", "check": result})
		fmt.Println("FULL 8K CACHED SWITCH CHECK", string(result))
	}
	var fullCheck json.RawMessage
	if e = evaluateTerrainPromise(ctx, b, "window.terrainBenchmark.validateNative()", &fullCheck); e != nil {
		return e
	}
	checks = append(checks, map[string]any{"region": regions[firstRegion].Name, "focus": "8K ADT", "check": fullCheck})
	fmt.Println("FULL 8K TILE CHECK", string(fullCheck))
	// Exercise the actual keyboard handler and verify the full camera state survives.
	var before, after struct {
		Mode   string          `json:"mode"`
		Camera json.RawMessage `json:"camera"`
		Matrix []float64       `json:"matrix"`
	}
	if e = b.evaluate(ctx, "window.terrainBenchmark.state()", &before); e != nil {
		return e
	}
	if _, e = b.call(ctx, "Input.dispatchKeyEvent", map[string]string{"type": "keyDown", "key": "ArrowRight", "code": "ArrowRight"}); e != nil {
		return e
	}
	if e = b.evaluate(ctx, "window.terrainBenchmark.state()", &after); e != nil {
		return e
	}
	if after.Mode != "wc3" || !reflect.DeepEqual(before.Camera, after.Camera) || !reflect.DeepEqual(before.Matrix, after.Matrix) {
		return fmt.Errorf("arrow toggle changed the camera or failed to switch mesh")
	}
	if _, e = b.call(ctx, "Input.dispatchKeyEvent", map[string]string{"type": "keyDown", "key": "ArrowLeft", "code": "ArrowLeft"}); e != nil {
		return e
	}
	if e = b.evaluate(ctx, "window.terrainBenchmark.state()", &after); e != nil {
		return e
	}
	if after.Mode != "wow" {
		return fmt.Errorf("left arrow did not select reference")
	}
	for _, result := range []struct {
		name string
		rows []map[string]any
	}{{"views.json", records}, {"native-checks.json", checks}, {"switch-checks.json", switches}} {
		if e = saveTerrainChecks(filepath.Join(out, result.name), onlyRegion, result.rows); e != nil {
			return e
		}
	}
	if e = evaluateTerrainPromise(ctx, b, fmt.Sprintf("window.terrainBenchmark.select(%d)", firstRegion), nil); e != nil {
		return e
	}
	if comparisonRegions[regions[firstRegion].Name] {
		if e = evaluateTerrainPromise(ctx, b, terrainSwitchCheck, nil); e != nil {
			return e
		}
	}
	if e = b.evaluate(ctx, "document.getElementById('wc3').click()", nil); e != nil {
		return e
	}
	raw, e := b.call(ctx, "Page.captureScreenshot", map[string]string{"format": "png"})
	if e != nil {
		return e
	}
	var shot struct {
		Data string `json:"data"`
	}
	if e = json.Unmarshal(raw, &shot); e != nil {
		return e
	}
	png, e := base64.StdEncoding.DecodeString(shot.Data)
	if e != nil {
		return e
	}
	return os.WriteFile(filepath.Join(out, "viewer.png"), png, 0644)
}

// A focused evaluation replaces only that region's evidence.
func saveTerrainChecks(path, onlyRegion string, rows []map[string]any) error {
	if onlyRegion != "" {
		if raw, err := os.ReadFile(path); err == nil {
			var previous []map[string]any
			if err = json.Unmarshal(raw, &previous); err != nil {
				return err
			}
			kept := make([]map[string]any, 0, len(previous)+len(rows))
			for _, row := range previous {
				if row["region"] != onlyRegion {
					kept = append(kept, row)
				}
			}
			rows = append(kept, rows...)
		} else if !os.IsNotExist(err) {
			return err
		}
	}
	raw, err := json.MarshalIndent(rows, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, raw, 0644)
}

// Exercise the actual buttons after preloading, with fetch/GPU allocation traps.
const terrainSwitchCheck = `(async()=>{
 const api=window.terrainBenchmark,deadline=performance.now()+45000;
 while(!['export-48','export-63'].every(stage=>api.state().cachedStages.includes(stage))){
  if(!api.state().ready||api.state().loading)throw Error('Preloading obscured the current terrain');
  if(performance.now()>deadline)throw Error('Comparison preload did not complete');
  await new Promise(resolve=>setTimeout(resolve,20));
 }
 document.getElementById('wc3').click();
 const before=api.state(),camera=JSON.stringify(before.camera),matrix=JSON.stringify(before.matrix);
 const oldFetch=window.fetch,methods=['createTexture','createBuffer','createVertexArray'],oldMethods=methods.map(k=>gl[k]);
 let requests=0,allocations=0,maxClickMs=0,hashes={};
 window.fetch=function(...args){requests++;return oldFetch.apply(this,args)};
 methods.forEach((key,i)=>gl[key]=function(...args){allocations++;return oldMethods[i].apply(this,args)});
 try{
  for(const [stage,button] of [['export-48','paint48'],['export-63','paint63'],['export-48','paint48'],['export-63','paint63'],['export-48','paint48']]){
   const start=performance.now();document.getElementById(button).click();maxClickMs=Math.max(maxClickMs,performance.now()-start);
   const state=api.state();
   if(state.stage!==stage||!state.ready||state.loading)throw Error('Budget switch was not synchronous');
   if(JSON.stringify(state.camera)!==camera||JSON.stringify(state.matrix)!==matrix)throw Error('Budget switch moved the camera');
   const pixels=new Uint8Array(canvas.width*canvas.height*4);gl.readPixels(0,0,canvas.width,canvas.height,gl.RGBA,gl.UNSIGNED_BYTE,pixels);
   let hash=2166136261;for(let i=0;i<pixels.length;i++)hash=Math.imul(hash^pixels[i],16777619);
   if(hashes[stage]!==undefined&&hashes[stage]!==hash)throw Error('Cached terrain frame changed on return');
   hashes[stage]=hash;
  }
  if(requests||allocations)throw Error('Cached switch fetched or rebuilt assets');
  if(hashes['export-48']===hashes['export-63'])throw Error('Budget buttons did not change the rendered terrain');
  return {switches:5,requests,allocations,maxClickMs,cameraUnchanged:true,loadingOverlay:false,distinctFrames:true};
 }finally{window.fetch=oldFetch;methods.forEach((key,i)=>gl[key]=oldMethods[i])}
})()`

func evaluateTerrainPromise(ctx context.Context, b *browser, expression string, result any) error {
	script := `(()=>{window.__terrainResult=undefined;window.__terrainFailure='';Promise.resolve().then(()=>` + expression + `).then(v=>window.__terrainResult=v).catch(e=>window.__terrainFailure=String(e));return true})()`
	if e := b.evaluate(ctx, script, nil); e != nil {
		return e
	}
	deadline := time.Now().Add(60 * time.Second)
	for {
		var status struct {
			Ready   bool   `json:"ready"`
			Failure string `json:"failure"`
		}
		if e := b.evaluate(ctx, `({ready:window.__terrainResult!==undefined,failure:window.__terrainFailure})`, &status); e != nil {
			return e
		}
		if status.Failure != "" {
			return fmt.Errorf("terrain capture: %s", status.Failure)
		}
		if status.Ready {
			return b.evaluate(ctx, "window.__terrainResult", result)
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("terrain capture timed out")
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(200 * time.Millisecond):
		}
	}
}
