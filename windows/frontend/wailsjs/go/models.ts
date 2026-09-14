export namespace main {
	
	export class SiteStatus {
	    domain: string;
	    code: number;
	    ok: boolean;
	    challenge: boolean;
	    detail: string;
	    checkedAt: string;
	
	    static createFrom(source: any = {}) {
	        return new SiteStatus(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.domain = source["domain"];
	        this.code = source["code"];
	        this.ok = source["ok"];
	        this.challenge = source["challenge"];
	        this.detail = source["detail"];
	        this.checkedAt = source["checkedAt"];
	    }
	}
	export class AdaptiveStatus {
	    sites: SiteStatus[];
	    warpDomains: string[];
	    note: string;
	
	    static createFrom(source: any = {}) {
	        return new AdaptiveStatus(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.sites = this.convertValues(source["sites"], SiteStatus);
	        this.warpDomains = source["warpDomains"];
	        this.note = source["note"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class ProbeStepView {
	    name: string;
	    status: string;
	    detail: string;
	
	    static createFrom(source: any = {}) {
	        return new ProbeStepView(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.status = source["status"];
	        this.detail = source["detail"];
	    }
	}
	export class ProbeReportView {
	    ok: boolean;
	    steps: ProbeStepView[];
	
	    static createFrom(source: any = {}) {
	        return new ProbeReportView(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ok = source["ok"];
	        this.steps = this.convertValues(source["steps"], ProbeStepView);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class ChannelView {
	    id: string;
	    transport: string;
	    server: string;
	    port: number;
	    validation: string;
	    enabled: boolean;
	
	    static createFrom(source: any = {}) {
	        return new ChannelView(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.transport = source["transport"];
	        this.server = source["server"];
	        this.port = source["port"];
	        this.validation = source["validation"];
	        this.enabled = source["enabled"];
	    }
	}
	export class AppState {
	    state: string;
	    blockedReason: string;
	    activeId: string;
	    active?: ChannelView;
	    proxyMode: string;
	    probe?: ProbeReportView;
	    probeRunning: boolean;
	    probeLastAt: string;
	    error: string;
	    coreReady: boolean;
	    coreBlockMsg: string;
	
	    static createFrom(source: any = {}) {
	        return new AppState(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.state = source["state"];
	        this.blockedReason = source["blockedReason"];
	        this.activeId = source["activeId"];
	        this.active = this.convertValues(source["active"], ChannelView);
	        this.proxyMode = source["proxyMode"];
	        this.probe = this.convertValues(source["probe"], ProbeReportView);
	        this.probeRunning = source["probeRunning"];
	        this.probeLastAt = source["probeLastAt"];
	        this.error = source["error"];
	        this.coreReady = source["coreReady"];
	        this.coreBlockMsg = source["coreBlockMsg"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	
	export class ConnView {
	    host: string;
	    network: string;
	    chain: string;
	    process: string;
	    up: number;
	    down: number;
	    since: string;
	
	    static createFrom(source: any = {}) {
	        return new ConnView(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.host = source["host"];
	        this.network = source["network"];
	        this.chain = source["chain"];
	        this.process = source["process"];
	        this.up = source["up"];
	        this.down = source["down"];
	        this.since = source["since"];
	    }
	}
	export class DeliveryProfile {
	    present: boolean;
	    version: number;
	    keyId: string;
	    expiresAt: string;
	    trustedKeys: number;
	    channelsApplied: number;
	    error: string;
	
	    static createFrom(source: any = {}) {
	        return new DeliveryProfile(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.present = source["present"];
	        this.version = source["version"];
	        this.keyId = source["keyId"];
	        this.expiresAt = source["expiresAt"];
	        this.trustedKeys = source["trustedKeys"];
	        this.channelsApplied = source["channelsApplied"];
	        this.error = source["error"];
	    }
	}
	export class FailoverStatus {
	    enabled: boolean;
	    state: string;
	    switches: number;
	    lastError: string;
	    nextCheckIn: number;
	
	    static createFrom(source: any = {}) {
	        return new FailoverStatus(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.enabled = source["enabled"];
	        this.state = source["state"];
	        this.switches = source["switches"];
	        this.lastError = source["lastError"];
	        this.nextCheckIn = source["nextCheckIn"];
	    }
	}
	export class LogLine {
	    t: string;
	    level: string;
	    text: string;
	
	    static createFrom(source: any = {}) {
	        return new LogLine(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.t = source["t"];
	        this.level = source["level"];
	        this.text = source["text"];
	    }
	}
	export class MetricsView {
	    available: boolean;
	    note?: string;
	    rateUp: number;
	    rateDown: number;
	    sessionUp: number;
	    sessionDown: number;
	    // Go type: time
	    at: any;
	    connections: ConnView[];
	
	    static createFrom(source: any = {}) {
	        return new MetricsView(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.available = source["available"];
	        this.note = source["note"];
	        this.rateUp = source["rateUp"];
	        this.rateDown = source["rateDown"];
	        this.sessionUp = source["sessionUp"];
	        this.sessionDown = source["sessionDown"];
	        this.at = this.convertValues(source["at"], null);
	        this.connections = this.convertValues(source["connections"], ConnView);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class NetGuardEvent {
	    time: string;
	    kind: string;
	    text: string;
	
	    static createFrom(source: any = {}) {
	        return new NetGuardEvent(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.time = source["time"];
	        this.kind = source["kind"];
	        this.text = source["text"];
	    }
	}
	export class NetGuardProxy {
	    enabled: boolean;
	    server: string;
	    port: number;
	    listenerAlive: boolean;
	    stale: boolean;
	
	    static createFrom(source: any = {}) {
	        return new NetGuardProxy(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.enabled = source["enabled"];
	        this.server = source["server"];
	        this.port = source["port"];
	        this.listenerAlive = source["listenerAlive"];
	        this.stale = source["stale"];
	    }
	}
	export class NetGuardTask {
	    name: string;
	    status: string;
	    lastRun: string;
	    nextRun: string;
	    lastCode: string;
	
	    static createFrom(source: any = {}) {
	        return new NetGuardTask(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.status = source["status"];
	        this.lastRun = source["lastRun"];
	        this.nextRun = source["nextRun"];
	        this.lastCode = source["lastCode"];
	    }
	}
	export class NetGuardStatus {
	    tasks: NetGuardTask[];
	    proxy: NetGuardProxy;
	    socksAlive: boolean;
	    events: NetGuardEvent[];
	    lastRun: string;
	    dohDisabled: boolean;
	
	    static createFrom(source: any = {}) {
	        return new NetGuardStatus(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.tasks = this.convertValues(source["tasks"], NetGuardTask);
	        this.proxy = this.convertValues(source["proxy"], NetGuardProxy);
	        this.socksAlive = source["socksAlive"];
	        this.events = this.convertValues(source["events"], NetGuardEvent);
	        this.lastRun = source["lastRun"];
	        this.dohDisabled = source["dohDisabled"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	
	export class NetStats {
	    inOctets: number;
	    outOctets: number;
	    speedBps: number;
	    alias: string;
	
	    static createFrom(source: any = {}) {
	        return new NetStats(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.inOctets = source["inOctets"];
	        this.outOctets = source["outOctets"];
	        this.speedBps = source["speedBps"];
	        this.alias = source["alias"];
	    }
	}
	export class NetworkFacts {
	    connectionType: string;
	    ssid: string;
	    localIP: string;
	    isp: string;
	    publicIP: string;
	    country: string;
	    countryOrigin: string;
	    dnsViaTunnel: boolean;
	    checkedAt: string;
	
	    static createFrom(source: any = {}) {
	        return new NetworkFacts(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.connectionType = source["connectionType"];
	        this.ssid = source["ssid"];
	        this.localIP = source["localIP"];
	        this.isp = source["isp"];
	        this.publicIP = source["publicIP"];
	        this.country = source["country"];
	        this.countryOrigin = source["countryOrigin"];
	        this.dnsViaTunnel = source["dnsViaTunnel"];
	        this.checkedAt = source["checkedAt"];
	    }
	}
	export class OnboardingKeyPreview {
	    key_id: string;
	    fingerprint: string;
	    comment?: string;
	
	    static createFrom(source: any = {}) {
	        return new OnboardingKeyPreview(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.key_id = source["key_id"];
	        this.fingerprint = source["fingerprint"];
	        this.comment = source["comment"];
	    }
	}
	export class OnboardingPreview {
	    created_at: string;
	    device_name?: string;
	    channels: number;
	    secrets: number;
	    split_direct: number;
	    keys: OnboardingKeyPreview[];
	    bundle_version: number;
	    current_version: number;
	    would_downgrade: boolean;
	
	    static createFrom(source: any = {}) {
	        return new OnboardingPreview(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.created_at = source["created_at"];
	        this.device_name = source["device_name"];
	        this.channels = source["channels"];
	        this.secrets = source["secrets"];
	        this.split_direct = source["split_direct"];
	        this.keys = this.convertValues(source["keys"], OnboardingKeyPreview);
	        this.bundle_version = source["bundle_version"];
	        this.current_version = source["current_version"];
	        this.would_downgrade = source["would_downgrade"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class OnboardingQRResult {
	    transport: string;
	    dataUrl: string;
	
	    static createFrom(source: any = {}) {
	        return new OnboardingQRResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.transport = source["transport"];
	        this.dataUrl = source["dataUrl"];
	    }
	}
	
	
	export class SecretTestReport {
	    ok: boolean;
	    steps: ProbeStepView[];
	
	    static createFrom(source: any = {}) {
	        return new SecretTestReport(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ok = source["ok"];
	        this.steps = this.convertValues(source["steps"], ProbeStepView);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	
	export class TestResultView {
	    id: string;
	    ok?: boolean;
	    detail: string;
	    checkedAt: string;
	    steps: ProbeStepView[];
	
	    static createFrom(source: any = {}) {
	        return new TestResultView(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.ok = source["ok"];
	        this.detail = source["detail"];
	        this.checkedAt = source["checkedAt"];
	        this.steps = this.convertValues(source["steps"], ProbeStepView);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class UpdatePreview {
	    current_version: string;
	    version: string;
	    notes?: string;
	    released_at?: string;
	    size: number;
	    sha256: string;
	    key_id: string;
	    floor: string;
	    ok: boolean;
	    reason?: string;
	
	    static createFrom(source: any = {}) {
	        return new UpdatePreview(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.current_version = source["current_version"];
	        this.version = source["version"];
	        this.notes = source["notes"];
	        this.released_at = source["released_at"];
	        this.size = source["size"];
	        this.sha256 = source["sha256"];
	        this.key_id = source["key_id"];
	        this.floor = source["floor"];
	        this.ok = source["ok"];
	        this.reason = source["reason"];
	    }
	}

}

export namespace secretvault {
	
	export class Meta {
	    id: string;
	    kind: string;
	    title: string;
	    hint: string;
	    backend: string;
	    storedValue: string;
	    fingerprint: string;
	    createdAt: string;
	    updatedAt: string;
	    lastVerifiedAt: string;
	    verifyStatus: string;
	    verifyError: string;
	    reveals: number;
	
	    static createFrom(source: any = {}) {
	        return new Meta(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.kind = source["kind"];
	        this.title = source["title"];
	        this.hint = source["hint"];
	        this.backend = source["backend"];
	        this.storedValue = source["storedValue"];
	        this.fingerprint = source["fingerprint"];
	        this.createdAt = source["createdAt"];
	        this.updatedAt = source["updatedAt"];
	        this.lastVerifiedAt = source["lastVerifiedAt"];
	        this.verifyStatus = source["verifyStatus"];
	        this.verifyError = source["verifyError"];
	        this.reveals = source["reveals"];
	    }
	}

}

