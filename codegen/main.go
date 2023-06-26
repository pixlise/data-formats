package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

type msgTypes struct {
	Req        bool
	Resp       bool
	Upd        bool
	SourceFile string
}

const protoMessagePrefix = "message "

func main() {
	var protoPath string
	var angularOutPath string
	var goOutPath string

	flag.StringVar(&protoPath, "protoPath", "", "Path to protobuf file")
	flag.StringVar(&angularOutPath, "angularOutPath", "", "Path to output angular code")
	flag.StringVar(&goOutPath, "goOutPath", "", "Path to output Go code")

	flag.Parse()

	w, _ := os.Getwd()
	fmt.Println("PIXLISE protobuf messaging helper code generator")
	fmt.Println("================================================")
	fmt.Printf("Running in: %v\n", w)

	// Delete previously output files in case we fail at some point, don't want user to think all is OK and compile and forget...
	angularFilePath := ""
	if len(angularOutPath) > 0 {
		angularFilePath = filepath.Join(angularOutPath, "wsMessageHandler.ts")
		os.Remove(angularFilePath)
	}

	goFilePath := ""
	if len(goOutPath) > 0 {
		goFilePath = filepath.Join(goOutPath, "wsMessage.go")
		os.Remove(goFilePath)
	}

	// Read all files
	files, err := ioutil.ReadDir(protoPath)
	if err != nil {
		log.Fatal(err)
	}

	messages := map[string]msgTypes{}
	for _, file := range files {
		filePath := filepath.Join(protoPath, file.Name())
		if strings.HasSuffix(filePath, ".proto") {
			protoLines := readProtoFile(filePath, &messages)

			// Scan to make sure all responses have a status field
			scanResponses(filePath, protoLines)
		}
	}

	// Make a sorted list so we write stuff in consistant order
	sortedMsgs := []string{}
	for msgName := range messages {
		sortedMsgs = append(sortedMsgs, msgName)
	}
	sort.Strings(sortedMsgs)

	checkMessageConsistancy(messages)
	checkWSMessage(filepath.Join(protoPath, "websocket.proto"), sortedMsgs, messages)

	// Make a list of all msgs
	allMsgTypes := listAllMessageTypes(sortedMsgs, messages)

	// Write out the code that handles all of these
	if len(angularOutPath) > 0 {
		err := os.MkdirAll(angularOutPath, 0644)
		if err != nil {
			log.Fatalf("Failed to create angular path: %v", err)
		}

		fmt.Printf("Writing Angular file to: %v\n", angularFilePath)
		writeAngular(allMsgTypes, sortedMsgs, messages, angularFilePath)
	} else {
		fmt.Println("Skipping Angular writing...")
	}

	if len(goOutPath) > 0 {
		err := os.MkdirAll(goOutPath, 0644)
		if err != nil {
			log.Fatalf("Failed to create go path: %v", err)
		}

		fmt.Printf("Writing Go file to: %v\n", goFilePath)
		writeGo(allMsgTypes, sortedMsgs, messages, goFilePath)
	} else {
		fmt.Println("Skipping Angular writing...")
	}

	// Check that Go handler functions are all there
	generateGoHandlers(sortedMsgs, messages, goOutPath)
}

func checkWSMessage(wsMsgFileName string, sortedMsgs []string, messages map[string]msgTypes) {
	// Read the existing file...
	proto, err := os.ReadFile(wsMsgFileName)

	if err != nil {
		log.Fatalf("Failed to read proto file containing WSMessage: %v. Error: %v", wsMsgFileName, err)
	}

	// Scan for lines that define messages
	protoLines := strings.Split(string(proto), "\n")

	msgProtoIds := map[string]int{}
	startRow := -1
	msgLinesRead := []string{}
	for c, line := range protoLines {
		if strings.Contains(line, "oneof Contents") {
			startRow = c + 2 // Assume next one is just "{"
		}

		if startRow < 0 {
			continue
		}

		if c == startRow-1 {
			// Verify it's just {
			if strings.Trim(line, "\t ") != "{" {
				log.Fatalf("Expected only '{' in %v, on line: %v", wsMsgFileName, c+1)
			}
		}

		if c >= startRow {
			trimmedLine := strings.Trim(line, "\t ")
			// We're scanning until }
			if trimmedLine == "}" {
				break
			}

			// Otherwise, assume we have a message def, we need its index
			// Assume line is like this:
			// ScanListResp scanListResp = 7;

			lineParts := strings.Split(trimmedLine, " ")
			if len(lineParts) != 4 {
				log.Fatalf("Unexpected syntax in %v, on line: %v. Line was: \"%v\"", wsMsgFileName, c+1, line)
			}

			// First part should be msg name, last part should be a number with a ; after
			numPart := lineParts[len(lineParts)-1]
			if !strings.HasSuffix(numPart, ";") {
				log.Fatalf("Expected ; at last part of line in %v, on line: %v. Line was: \"%v\"", wsMsgFileName, c+1, line)
			}

			numPart = numPart[0 : len(numPart)-1]
			num, err := strconv.Atoi(numPart)
			if err != nil {
				log.Fatalf("Expected number in last part of line in %v, on line: %v. Line was: \"%v\"", wsMsgFileName, c+1, line)
			}

			msgProtoIds[lineParts[0]] = num
			msgLinesRead = append(msgLinesRead, trimmedLine)
		}
	}

	exampleLines := []string{}
	for _, msg := range sortedMsgs {
		msgType := messages[msg]
		toWrite := []string{}
		if msgType.Req {
			toWrite = append(toWrite, msg+"Req")
		}
		if msgType.Resp {
			toWrite = append(toWrite, msg+"Resp")
		}
		if msgType.Upd {
			toWrite = append(toWrite, msg+"Upd")
		}

		for _, msgName := range toWrite {
			idStr := ""
			if id, ok := msgProtoIds[msgName]; ok {
				idStr = fmt.Sprintf("%v", id)
			}
			exampleLines = append(exampleLines, fmt.Sprintf("        "+msgName+" "+varName(msgName)+" = "+idStr+";"))
		}
	}

	// Alphabetical order, so comparison is more useful!
	sort.Strings(exampleLines)

	// Print only if they differ...
	printMsg := false
	if len(msgLinesRead) != len(exampleLines) {
		printMsg = true
	} else {
		for c, read := range msgLinesRead {
			if read != strings.Trim(exampleLines[c], "\t ") {
				printMsg = true
				break
			}
		}
	}

	if !printMsg {
		return
	}

	// Show a list of all messages that we'd expect to be in WSMessage's "oneof" field
	fmt.Println("----------------------------------------------------------------------")
	fmt.Println("WSMessage should have the following messages in its Contents field...")
	fmt.Println("!!! Take care when updating, you don't want to redefine any of the IDs !!!")

	for _, line := range exampleLines {
		fmt.Println(line)
	}

	fmt.Println("----------------------------------------------------------------------")
}

// With help from:
// https://appliedgo.com/blog/a-tip-and-a-trick-when-working-with-generics
func writeGo(allMsgTypes []string, sortedMsgs []string, msgs map[string]msgTypes, goOutPath string) {
	goFunc := `package ws
	
// GENERATED CODE! Do not hand-modify

import (
	protos "github.com/pixlise/core/v3/generated-protos"
	"github.com/olahol/melody"
	"fmt"
	wsHandler "github.com/pixlise/core/v3/api/ws/handlers"
)	
`
	/*	goFunc += `
		// This lost type-safety because Go compiler said:
		// "cannot handle more than 100 union terms (implementation limitation)"
		// So we now allow passing an any :(
		//func MakeWSMessage[T protos.` + strings.Join(allMsgTypes, "|protos.") + `](msg *T) *protos.WSMessage {
		func MakeWSMessage(msg any) *protos.WSMessage {

			wsmsg := protos.WSMessage{}

			switch any(msg).(type) {
		`
			for _, msgType := range allMsgTypes {
				goFunc += "    case *protos." + msgType + ":\n        wsmsg.Contents = &protos.WSMessage_" + msgType + "{" + msgType + ": (msg.(*protos." + msgType + "))}\n"
			}

			goFunc += "    }\n    return &wsmsg\n}"
	*/

	allUpdMsgs := []string{}
	for _, name := range sortedMsgs {
		types := msgs[name]
		if types.Upd {
			allUpdMsgs = append(allUpdMsgs, name+"Upd")
		}
	}
	sort.Strings(allUpdMsgs)

	goFunc += `
func MakeUpdateWSMessage[T protos.` + strings.Join(allUpdMsgs, "|protos.") + `](msg *T) *protos.WSMessage {
	wsmsg := protos.WSMessage{}

	switch any(msg).(type) {
`
	for _, msgType := range allUpdMsgs {
		goFunc += "    case *protos." + msgType + ":\n        wsmsg.Contents = &protos.WSMessage_" + msgType + "{" + msgType + ": (any(msg).(*protos." + msgType + "))}\n"
	}

	goFunc += "    }\n    return &wsmsg\n}"

	goFunc += `

func (ws *WSHandler) dispatchWSMessage(wsmsg *protos.WSMessage, s *melody.Session) (*protos.WSMessage, error) {
	switch wsmsg.Contents.(type) {
`
	for _, name := range sortedMsgs {
		types := msgs[name]
		if types.Req {
			goFunc += fmt.Sprintf(`        case *protos.WSMessage_%vReq:
            resp, err := wsHandler.Handle%vReq(wsmsg.Get%vReq(), s, ws.melody, ws.svcs)
			if resp == nil || err != nil {
                return &protos.WSMessage{Contents: &protos.WSMessage_%vResp{%vResp: &protos.%vResp{}}, Status: makeRespStatus(err), ErrorText: err.Error()}, nil
			}
			return &protos.WSMessage{Contents: &protos.WSMessage_%vResp{%vResp: resp}}, nil				
`, name, name, name, name, name, name, name, name)
		}
	}

	goFunc += `        default:
		    return nil, fmt.Errorf("Unhandled message type: %v", wsmsg.String())
	}
}
`
	err := os.WriteFile(goOutPath, []byte(goFunc), 0644)
	if err != nil {
		log.Fatalf("Failed to write Go file: %v", err)
	}
}

func generateGoHandlers(sortedMsgs []string, msgs map[string]msgTypes, goOutPath string) {
	type outFileInfo struct {
		filePath         string
		existed          bool
		generatedContent string
		signatures       []string
	}

	handlerPath := filepath.Join(goOutPath, "handlers")
	err := os.MkdirAll(handlerPath, 0644)
	if err != nil {
		log.Fatalf("Failed to create handler path: %v. Error: %v", handlerPath, err)
	}

	// Loop through all messages, check that handler files exist, if not, generate, otherwise show what's missing
	files := map[string]outFileInfo{}
	for _, msgName := range sortedMsgs {
		types := msgs[msgName]
		if types.Req {
			if _, ok := files[types.SourceFile]; !ok {
				// New entry...
				suffixToReplace := ".proto"
				if strings.HasSuffix(types.SourceFile, "-msgs.proto") {
					suffixToReplace = "-msgs.proto"
				}
				outFile := types.SourceFile[0:len(types.SourceFile)-len(suffixToReplace)] + ".go"
				outPath := filepath.Join(handlerPath, outFile)

				fi, err := os.Stat(outPath)
				exists := err == nil && !fi.IsDir()

				files[types.SourceFile] = outFileInfo{
					filePath: outPath,
					existed:  exists,
				}
			}

			// Now that we know the out file struct exists, generate handler
			funcName := fmt.Sprintf("Handle%vReq", msgName)
			signature := fmt.Sprintf("func %v(req *protos.%vReq, s *melody.Session, m *melody.Melody, svcs *services.APIServices) (*protos.%vResp, error)", funcName, msgName, msgName)
			handler := signature + fmt.Sprintf(` {
    return nil, errors.New("%v not implemented yet")
}
`, funcName)
			existing := files[types.SourceFile]

			files[types.SourceFile] = outFileInfo{
				filePath:         existing.filePath,
				existed:          existing.existed,
				generatedContent: existing.generatedContent + handler,
				signatures:       append(existing.signatures, signature),
			}
		}
	}

	// Write out what we generated - if the file doesn't exist yet, create it, otherwise just write suggested content to stdout
	for _, finfo := range files {
		content := `package wsHandler

import (
	protos "github.com/pixlise/core/v3/generated-protos"
	"github.com/olahol/melody"
	"github.com/pixlise/core/v3/api/services"
	"errors"
)

`
		content += finfo.generatedContent

		if !finfo.existed {
			err := os.WriteFile(finfo.filePath, []byte(content), 0644)
			if err != nil {
				log.Fatalf("Failed when generating: %v. Error: %v", finfo.filePath, err)
			}
		} else {
			// Show suggestion for any missing signatures
			contents, err := os.ReadFile(finfo.filePath)
			if err != nil {
				log.Fatalf("Failed to open existing file: %v. Error: %v", finfo.filePath, err)
			}

			contentsStr := string(contents)

			missing := ""
			for _, sig := range finfo.signatures {
				if !strings.Contains(contentsStr, sig) {
					missing += sig + "\n"
				}
			}

			if len(missing) > 0 {
				fmt.Printf("%v should contain functions:\n", finfo.filePath)
				fmt.Printf("%v\n", missing)
			}
		}
	}
}

func varName(name string) string {
	name = strings.ToLower(name[0:1]) + name[1:]
	return name
}

func writeAngular(allMsgTypes []string, sortedMsgs []string, msgs map[string]msgTypes, angularOutPath string) {
	angular := `// GENERATED CODE! Do not hand-modify

import { Subject } from 'rxjs';
`
	sourceImports := map[string][]string{}
	sourceFiles := []string{}

	for _, name := range sortedMsgs {
		types := msgs[name]
		if _, ok := sourceImports[types.SourceFile]; !ok {
			sourceImports[types.SourceFile] = []string{}
			sourceFiles = append(sourceFiles, types.SourceFile)
		}

		toAdd := []string{}
		if types.Req {
			toAdd = append(toAdd, name+"Req")
		}
		if types.Resp {
			toAdd = append(toAdd, name+"Resp")
		}
		if types.Upd {
			toAdd = append(toAdd, name+"Upd")
		}

		for _, add := range toAdd {
			sourceImports[types.SourceFile] = append(sourceImports[types.SourceFile], add)
		}
	}

	sort.Strings(sourceFiles)
	for _, f := range sourceFiles {
		is := sourceImports[f]
		f = f[0 : len(f)-len(".proto")]
		angular += "import {\n    " + strings.Join(is, ",\n    ") + ` } from "src/app/generated-protos/` + f + "\"\n"
	}

	angular += `import { WSMessage } from "src/app/generated-protos/websocket"
import { ResponseStatus } from "src/app/generated-protos/response"

export class WSError
{
	constructor(public status: ResponseStatus, public errorText: string)
	{
	}
}

// Type-specific request send functions which return the right type of response
export abstract class WSMessageHandler
{
    private _lastMsgId = 1;

	protected abstract sendRequest(msg: WSMessage): void;

`
	for _, name := range sortedMsgs {
		types := msgs[name]
		if types.Upd {
			angular += fmt.Sprintf("    public %vUpd$ = new Subject<%vUpd>();\n", varName(name), name)
		}
	}

	for _, name := range sortedMsgs {
		types := msgs[name]
		// If there is a request and response pair, generate a function for it
		if types.Req && types.Resp {
			angular += fmt.Sprintf("\n    protected _%vSubjects = new Map<number, Subject<%vResp>>();\n", name, name)
			angular += fmt.Sprintf(`    send%vRequest(req: %vReq): Subject<%vResp> {
        let wsreq = WSMessage.create({%vReq: req});
        wsreq.msgId = this._lastMsgId++;

		let subj = new Subject<%vResp>();
        this._%vSubjects.set(wsreq.msgId, subj);
        this.sendRequest(wsreq);

		return subj;
    }
`, name, name, name, varName(name), name, name)
		}
	}

	angular += `
    protected dispatchResponse(wsmsg: WSMessage): boolean {
`
	firstIf := true
	for _, name := range sortedMsgs {
		types := msgs[name]
		if types.Resp {
			angular += "        "
			if !firstIf {
				angular += "else "
			}
			firstIf = false

			angular += fmt.Sprintf(`if(wsmsg.%vResp) {
            let subj = this._%vSubjects.get(wsmsg.msgId);
		    if(subj) {
				if(wsmsg.status != ResponseStatus.WS_OK) {
					subj.error(new WSError(wsmsg.status, wsmsg.errorText));
				} else {
			    	subj.next(wsmsg.%vResp);
				}
			    subj.complete();

			    this._%vSubjects.delete(wsmsg.msgId);
				return true;
		    }
        }
`, varName(name), name, varName(name), name)
		}
	}

	angular += `
        return false;
	}
`

	angular += `
    protected dispatchUpdate(wsmsg: WSMessage): boolean {
`
	firstIf = true
	for _, name := range sortedMsgs {
		types := msgs[name]
		if types.Upd {
			angular += "        "
			if !firstIf {
				angular += "else "
			}
			firstIf = false

			angular += fmt.Sprintf(`if(wsmsg.%vUpd) {
            this.%vUpd$.next(wsmsg.%vUpd);
			return true;
        }
`, varName(name), varName(name), varName(name))
		}
	}

	angular += `
        return false;
	}
}`

	err := os.WriteFile(angularOutPath, []byte(angular), 0644)
	if err != nil {
		log.Fatalf("Failed to write angular file: %v", err)
	}
}

func scanResponses(fileName string, protoLines []string) {
	scanCount := 0

	for lineCount, line := range protoLines {
		if scanCount <= 0 {
			if strings.HasPrefix(line, protoMessagePrefix) && strings.HasSuffix(line, "Resp") {
				// We have a line like: "message UserDetailsResp"
				// Expect following 2 lines to be:
				// {
				//     ResponseStatus status = 1;
				scanCount = 1 // We scanned the first part of this...
			}
		} else if scanCount == 1 {
			if strings.TrimSpace(line) != "{" {
				log.Fatalf("Expected { in %v, line: %v for message definition: %v\n", fileName, lineCount+1, protoLines[lineCount-scanCount])
			} else {
				scanCount++
			}
		} else if scanCount == 2 {
			if strings.TrimSpace(line) != "ResponseStatus status = 1;" {
				log.Fatalf("Expected \"ResponseStatus status = 1;\" in %v, line: %v for message definition: %v\n", fileName, lineCount+1, protoLines[lineCount-scanCount])
			} else {
				scanCount = 0
			}
		}
	}
}

func readProtoFile(protoPath string, messages *map[string]msgTypes) []string {
	proto, err := os.ReadFile(protoPath)

	if err != nil {
		log.Fatalf("Failed to read proto file: %v. Error: %v", protoPath, err)
	}

	// Scan for lines that define messages
	protoLines := strings.Split(string(proto), "\n")

	blanking := false
	for _, line := range protoLines {
		// If we detect a /*, read them as blank lines until */
		if !blanking {
			idx := strings.Index(line, "/*")
			if idx >= 0 {
				blanking = true
				line = line[0:idx]
			}
		} else {
			idx := strings.Index(line, "*/")
			if idx < 0 {
				// blank out this line...
				line = ""
			} else {
				// We found the end marker
				blanking = false
				line = line[idx+2:]
			}
		}

		if strings.HasPrefix(line, protoMessagePrefix) {
			// Check what kind of message
			hasReq := strings.HasSuffix(line, "Req")
			hasResp := strings.HasSuffix(line, "Resp")
			hasUpd := strings.HasSuffix(line, "Upd")

			if hasReq || hasResp || hasUpd {
				suffixLen := 3
				if hasResp {
					suffixLen = 4
				}
				msgName := line[len(protoMessagePrefix) : len(line)-suffixLen]

				if _, ok := (*messages)[msgName]; !ok {
					(*messages)[msgName] = msgTypes{
						SourceFile: filepath.Base(protoPath),
					}
				}

				types := (*messages)[msgName]
				types.Req = types.Req || hasReq
				types.Resp = types.Resp || hasResp
				types.Upd = types.Upd || hasUpd

				(*messages)[msgName] = types
			}
		}
	}

	return protoLines
}

func listAllMessageTypes(sortedMsgs []string, messages map[string]msgTypes) []string {
	allMsgTypes := []string{}
	// Add each type we have, as all request, response and updates should be able to fit into a WSMessage
	for _, msg := range sortedMsgs {
		msgType := messages[msg]
		if msgType.Req {
			allMsgTypes = append(allMsgTypes, msg+"Req")
		}
		if msgType.Resp {
			allMsgTypes = append(allMsgTypes, msg+"Resp")
		}
		if msgType.Upd {
			allMsgTypes = append(allMsgTypes, msg+"Upd")
		}
	}

	return allMsgTypes
}

func checkMessageConsistancy(messages map[string]msgTypes) {
	// Check that where there is a message type defined, we have Req and Resp with optional Upd, but none of those exist alone
	for msgName, msgTypes := range messages {
		if msgTypes.Req && !msgTypes.Resp {
			log.Fatalf("Found message %v has req but no resp", msgName)
		}
		if !msgTypes.Req && msgTypes.Resp {
			log.Fatalf("Found message %v has resp but no req", msgName)
		}
		if msgTypes.Upd && (!msgTypes.Req || !msgTypes.Resp) {
			log.Fatalf("Found message %v has upd but missing req or resp", msgName)
		}
	}
}
