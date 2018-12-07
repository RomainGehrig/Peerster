Vue.component('message', {
    template: `<div>
                <div class="float-right"><strong>{{ message.origin }} - #{{ message.id }}</strong></div>
                {{ message.text }}
              </div>`,
    props: ["message"]
});

Vue.component('tab-content', {
    template: ` <div role="tabpanel" :id="destination" class="tab-pane" :class="{ 'active': active, show: active }">
                    <message v-for="(message, index) in messages" v-bind:message="message"></message>
                    <div class="hr-line-dashed"></div>
                    <div class="input-group">
                        <input type="text" :id="'messageText-' + destination" v-on:keyup="sendOnEnter(destination)($event)" class="form-control" placeholder="Type message...">
                        <span class="input-group-append">
                           <a :href="'#'+destination" class="btn btn-primary" onclick="clickSendMessage(this.hash)">Send message to {{destination}}</a>
                        </span>
                    </div>
                </div>`,
    props: ["destination", "messages", "active"],
    methods: {
        sendOnEnter: function(destination) {
            // TODO Clean this hash mess
            return onEnter(clickSendMessage, "#"+destination);
        }
    }
});

Vue.component('file-request', {
    template: `<a :id="'file-download-popover-'+destination" :href="'#'+destination" data-toggle="popover" class="btn btn-outline-info btn-sm"><i class="fas fa-file-download"></i>
               <div :id="'file-download-popover-head-' + destination" class="d-none">
                  Download file from {{destination}}
               </div>
               <div :id="'file-download-popover-content-' + destination" class="d-none">
                  <form onsubmit="clickSubmitFileRequest(this); return false;">
                    <input id="file-download-destination" type="hidden" :value="destination"/>
                    <input id="file-download-hash" class="form-control" type="text" placeholder="Enter file hash"></input>
                    <input id="file-download-filename" class="form-control" type="text" placeholder="Enter filename"></input>
                    <input type="submit" class="btn btn-info btn-block btn-big" value="Send request"/>
               </form>
               </div></a>`,
    props: ["destination"],
    mounted: function() {
        const component = this;
        // Enable popover
        $('#file-download-popover-' + this.destination).popover({
            html : true,
            title: function() {
                return $("#file-download-popover-head-" + component.destination).html();
            },
            content: function() {
                return $("#file-download-popover-content-" + component.destination).html();
            }
        });
    }
});

const RUMOR_TAB = "_rumors";

let vue = new Vue({
    el: "#vuejs",
    data: {
        // messages: [],
        nodes: [],
        peerID: "",
        destinations: [],
        activeTab: RUMOR_TAB,
        tabs: {[RUMOR_TAB]: []},
        files: [],
        selectedFile: "",
        search: null,
        searchResults: []
    },
    methods: {
        fileChange: function(name, files) {
            if (files.length > 0) {
                this.selectedFile = files[0].name;
            } else {
                this.selectedFile = "";
            }
        },

        // Copied from https://stackoverflow.com/a/34310051/1439561
        toHexString: function(byteArray) {
            return Array.from(byteArray, function(byte) {
                return ('0' + (byte & 0xFF).toString(16)).slice(-2);
            }).join('').toUpperCase();
        },

        humanSize: function(byteCount) {
            const units = ["B", "KB", "MB", "GB"];
            const precision = 1;

            let currUnit = 0;
            while (byteCount > 1024 && currUnit + 1 < units.length) {
                byteCount /= 1024;
                currUnit++;
            }

            const ceiledNum = Math.ceil(byteCount * 10**precision)/10**precision;
            let strNum = ceiledNum.toFixed(precision);

            // Show as integer if we are near it
            if (Math.abs(byteCount - ~~byteCount) < 0.05) {
                strNum = ~~byteCount;
            }
            return strNum + " " + units[currUnit];

        }
    }
});

function init() {
    const nodeAddr = document.getElementById("nodeValue");
    nodeAddr.addEventListener("keyup", onEnter(clickNewNode));

    // Enable all tooltips
    $('[data-toggle="tooltip"]').tooltip();

    refreshInfo();
    setInterval(refreshInfo, 3000);
}

function refreshInfo() {
    retrieveMessages();
    retrievePrivateMessages();
    retrieveNodes();
    retrievePeerID();
    retrieveDestinations();
    retrieveSharedFiles();
    retrieveSearchResults();
}


function compareStrings(s1, s2) {
    let as = s1.toLowerCase();
    let bs = s2.toLowerCase();
    if (as < bs) return -1;
    if (as > bs) return 1;
    return 0;
}

function sortMessages(msgs) {
    if (msgs === null)
        return [];

    msgs.sort((a, b) => {
        if (a["origin"] == b["origin"])
            return a["id"] - b["id"];
        else {
            return compareStrings(a["origin"], b["origin"]);
        }
    });

    return msgs;
}

function sortFiles(files) {
    if (files === null)
        return [];

    files.sort((a, b) => compareStrings(a['filename'], b['filename']));

    return files;
}

function wantToSendMessageTo(hash) {
    changeTab(hash);

    const dest = stripHash(hash, RUMOR_TAB);
    if (!(dest in vue.tabs)) {
        Vue.set(vue.tabs, dest, []);
    }
}

function stripHash(hash, defaultValue) {
    if (hash === "") {
        return defaultValue;
    } else {
        return hash.slice(1);
    }
}

function sortNodes(nodes) {
    if (nodes === null)
        return [];

    nodes.sort();
    return nodes;
}

function onEnter(func, ...args) {
    return function(event) {
        event.preventDefault();
        if (event.keyCode === 13) {
            func(...args);
        }
    };
}

function changeTab(hash) {
    vue.activeTab = stripHash(hash, RUMOR_TAB);
}

function clickSubmitFileRequest(form) {
    const fileDestElem = form["file-download-destination"];
    const fileHashElem = form["file-download-hash"];
    const fileNameElem = form["file-download-filename"];
    const destination = fileDestElem.value;
    const fileHash = fileHashElem.value;
    const fileName = fileNameElem.value;

    if (fileHash == "" || fileName == "") {
        return;
    }

    // TODO Show a message

    // TODO Do this without a shotgun
    $(".popover").popover("hide");
    sendFileRequest(destination, fileHash, fileName);
}

// Here we receive the index in the list of results
function clickDownloadOnSearchResult(index) {
    file = vue.searchResults[index];

    const alert = $("#alertDownload");
    alert.html("Starting download of <strong>" + file.filename + "</strong>");
    alert.fadeIn();
    setTimeout(() => alert.fadeOut(), 3000);

    // Destination == "" means the file download will use the search information to retrieve the file
    sendFileRequest(destination="", hash=file.hash, filename=file.filename);
}

function clickSearchButton(form) {
    const text = form["searchText"];
    const keywords = text.value.split(",");

    text.value = "";
    vue.search = keywords;
    vue.searchResults = [];
    sendSearchRequest(keywords);
}

function clickSendFile() {
    const filename = vue.selectedFile;
    vue.selectedFile = "";
    sendFileName(filename);
    refreshInfo();
}

function clickSendMessage(hash) {
    const destination = stripHash(hash, RUMOR_TAB);
    const elem = document.getElementById(`messageText-${destination}`);
    const text = elem.value;
    // Reset text
    elem.value = "";

    if (text !== "") {
        if (destination === RUMOR_TAB) {
            sendMessage(text);
        } else {
            sendPrivateMessage(text, destination);
        }
    }
    refreshInfo();
}

function clickNewNode() {
    const elem = document.getElementById("nodeValue");
    const addr = elem.value;
    elem.value = "";

    if (nodeAddrLooksValid(addr)) {
        addNode(addr);
        refreshInfo();
    } else {
        $("#nodeError").fadeIn();
        setTimeout(() => $("#nodeError").fadeOut(), 3000);
    }

}

function nodeAddrLooksValid(str) {
    const splitted = str.split(":");
    if (splitted.length !== 2)
        return false;

    const parsed = Number(splitted[1]);
    // Is not a number
    if (parsed === NaN)
        return false;

    // Is not an integer
    if (~~parsed !== parsed)
        return false;

    return true;
}

// Messages
async function sendMessage(msg) {
    return jsonPost("/message", JSON.stringify({"text": msg}));
}

async function retrieveMessages() {
    jsonGet("/message").then((obj) => Vue.set(vue.tabs, RUMOR_TAB, sortMessages(obj["messages"])));
}

async function sendPrivateMessage(msg, dest) {
    return jsonPost("/pmessage", JSON.stringify({"text": msg, "dest": dest}));
}

async function retrievePrivateMessages() {
    jsonGet("/pmessage").then((obj) => {
        for (origin in obj) {
            Vue.set(vue.tabs, origin, obj[origin]);
        }
    });
}


// ID
async function retrievePeerID() {
    jsonGet("/id").then((obj) => vue.peerID = obj["id"]);
}

// Destinations
async function retrieveDestinations() {
    jsonGet("/destinations").then((obj) => vue.destinations = sortNodes(obj["destinations"]));
}

// Shared files
async function retrieveSharedFiles() {
    jsonGet("/sharedFiles").then((obj) => vue.files = sortFiles(obj["files"]));
}

async function sendFileRequest(dest, hash, filename) {
    if (Array.isArray(hash)) {
        hash = vue.toHexString(hash);
    }
    jsonPost("/files", JSON.stringify({"destination": dest, "hash": hash, "filename": filename}));
}

// Nodes
async function retrieveNodes() {
    jsonGet("/node").then((obj) => vue.nodes = sortNodes(obj["nodes"]));
}
async function addNode(node) {
    return jsonPost("/node", JSON.stringify({"addr": node}));
}

// Files
async function sendFileName(filename) {
    return jsonPost("/sharedFiles", JSON.stringify({"filename": filename}));
}

// Search
async function sendSearchRequest(keywords) {
    jsonPost("/search", JSON.stringify({"keywords": keywords}));
}
async function retrieveSearchResults() {
    jsonGet("/search").then((obj) => {
        if (obj === null) {
            return;
        }
        vue.search = obj.keywords;
        vue.searchResults = obj.files || [];
    });
}

//// Helper methods
async function jsonQuery(method, endpoint, body) {
    return fetch(endpoint, {
        method: method,
        mode: 'cors',
        body: body,
        cache: "no-cache",
        headers: {
            "Accept": "application/json",
            "Content-Type": "application/json"
        }
    }).then((resp) => resp.json());
}

async function jsonGet(endpoint) {
    return jsonQuery("GET", endpoint, null);
}

async function jsonPost(endpoint, jsonData) {
    return jsonQuery("POST", endpoint, jsonData);
}
