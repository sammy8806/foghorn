export namespace config {
	
	export class ActionDef {
	    Type: string;
	    Template: string;
	    Command: string;
	    Terminal: boolean;
	
	    static createFrom(source: any = {}) {
	        return new ActionDef(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.Type = source["Type"];
	        this.Template = source["Template"];
	        this.Command = source["Command"];
	        this.Terminal = source["Terminal"];
	    }
	}
	export class ActionConfig {
	    Name: string;
	    Match: Record<string, string>;
	    Action: ActionDef;
	    Icon: string;
	
	    static createFrom(source: any = {}) {
	        return new ActionConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.Name = source["Name"];
	        this.Match = source["Match"];
	        this.Action = this.convertValues(source["Action"], ActionDef);
	        this.Icon = source["Icon"];
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
	
	export class BadgeRule {
	    label: string;
	    field: string;
	    equals: string[];
	    sources: string[];
	    source_types: string[];
	
	    static createFrom(source: any = {}) {
	        return new BadgeRule(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.label = source["label"];
	        this.field = source["field"];
	        this.equals = source["equals"];
	        this.sources = source["sources"];
	        this.source_types = source["source_types"];
	    }
	}
	export class DisplayPriority {
	    mode: string;
	    sources: string[];
	    source_types: string[];
	
	    static createFrom(source: any = {}) {
	        return new DisplayPriority(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.mode = source["mode"];
	        this.sources = source["sources"];
	        this.source_types = source["source_types"];
	    }
	}
	export class SortCriterion {
	    field: string;
	    order: string;
	
	    static createFrom(source: any = {}) {
	        return new SortCriterion(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.field = source["field"];
	        this.order = source["order"];
	    }
	}
	export class VisibleEntry {
	    source: string;
	    order: number;
	    label?: string;
	    style?: string[];
	
	    static createFrom(source: any = {}) {
	        return new VisibleEntry(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.source = source["source"];
	        this.order = source["order"];
	        this.label = source["label"];
	        this.style = source["style"];
	    }
	}
	export class NormalizedDisplayConfig {
	    visible_labels: VisibleEntry[];
	    visible_annotations: VisibleEntry[];
	    subtitle_annotations: string[];
	    group_by: string[];
	    group_by_override_key_mode: string;
	    group_by_overrides: Record<string, Array<string>>;
	    priority: DisplayPriority;
	    badges: BadgeRule[];
	    sort_by: SortCriterion[];
	
	    static createFrom(source: any = {}) {
	        return new NormalizedDisplayConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.visible_labels = this.convertValues(source["visible_labels"], VisibleEntry);
	        this.visible_annotations = this.convertValues(source["visible_annotations"], VisibleEntry);
	        this.subtitle_annotations = source["subtitle_annotations"];
	        this.group_by = source["group_by"];
	        this.group_by_override_key_mode = source["group_by_override_key_mode"];
	        this.group_by_overrides = source["group_by_overrides"];
	        this.priority = this.convertValues(source["priority"], DisplayPriority);
	        this.badges = this.convertValues(source["badges"], BadgeRule);
	        this.sort_by = this.convertValues(source["sort_by"], SortCriterion);
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
	export class NormalizedSeverityLevel {
	    name: string;
	    color: string;
	    aliases: string[];
	    rank: number;
	
	    static createFrom(source: any = {}) {
	        return new NormalizedSeverityLevel(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.color = source["color"];
	        this.aliases = source["aliases"];
	        this.rank = source["rank"];
	    }
	}
	export class NormalizedSeverityConfig {
	    default: string;
	    levels: NormalizedSeverityLevel[];
	
	    static createFrom(source: any = {}) {
	        return new NormalizedSeverityConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.default = source["default"];
	        this.levels = this.convertValues(source["levels"], NormalizedSeverityLevel);
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
	
	export class SilenceEditorConfig {
	    always_visible_matchers?: string[];
	    collapse_matchers?: boolean;
	
	    static createFrom(source: any = {}) {
	        return new SilenceEditorConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.always_visible_matchers = source["always_visible_matchers"];
	        this.collapse_matchers = source["collapse_matchers"];
	    }
	}
	
	export class UIScale {
	    factor: number;
	    mode: string;
	    apply_to_popup: boolean;
	
	    static createFrom(source: any = {}) {
	        return new UIScale(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.factor = source["factor"];
	        this.mode = source["mode"];
	        this.apply_to_popup = source["apply_to_popup"];
	    }
	}
	export class UIConfig {
	    theme: string;
	    popup_width: number;
	    popup_height: number;
	    popup_position: string;
	    auto_position?: boolean;
	    always_on_top?: boolean;
	    popup_follow_cursor?: boolean;
	    show_resolved: boolean;
	    show_silenced: boolean;
	    default_created_by: string;
	    idle_image: string;
	    scale: UIScale;
	    silence_editor: SilenceEditorConfig;
	
	    static createFrom(source: any = {}) {
	        return new UIConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.theme = source["theme"];
	        this.popup_width = source["popup_width"];
	        this.popup_height = source["popup_height"];
	        this.popup_position = source["popup_position"];
	        this.auto_position = source["auto_position"];
	        this.always_on_top = source["always_on_top"];
	        this.popup_follow_cursor = source["popup_follow_cursor"];
	        this.show_resolved = source["show_resolved"];
	        this.show_silenced = source["show_silenced"];
	        this.default_created_by = source["default_created_by"];
	        this.idle_image = source["idle_image"];
	        this.scale = this.convertValues(source["scale"], UIScale);
	        this.silence_editor = this.convertValues(source["silence_editor"], SilenceEditorConfig);
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
	

}

export namespace main {
	
	export class AboutInfo {
	    name: string;
	    version: string;
	    description: string;
	    repoURL: string;
	    copyright: string;
	
	    static createFrom(source: any = {}) {
	        return new AboutInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.version = source["version"];
	        this.description = source["description"];
	        this.repoURL = source["repoURL"];
	        this.copyright = source["copyright"];
	    }
	}

}

export namespace model {
	
	export class Matcher {
	    name: string;
	    value: string;
	    isRegex: boolean;
	    isEqual: boolean;
	
	    static createFrom(source: any = {}) {
	        return new Matcher(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.value = source["value"];
	        this.isRegex = source["isRegex"];
	        this.isEqual = source["isEqual"];
	    }
	}
	export class SilenceInfo {
	    id: string;
	    createdBy: string;
	    comment: string;
	    // Go type: time
	    startsAt: any;
	    // Go type: time
	    endsAt: any;
	    matchers: Matcher[];
	
	    static createFrom(source: any = {}) {
	        return new SilenceInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.createdBy = source["createdBy"];
	        this.comment = source["comment"];
	        this.startsAt = this.convertValues(source["startsAt"], null);
	        this.endsAt = this.convertValues(source["endsAt"], null);
	        this.matchers = this.convertValues(source["matchers"], Matcher);
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
	export class Alert {
	    id: string;
	    source: string;
	    sourceType: string;
	    name: string;
	    severity: string;
	    state: string;
	    labels: Record<string, string>;
	    annotations: Record<string, string>;
	    resolvedLabels?: Record<string, string>;
	    resolvedAnnotations?: Record<string, string>;
	    resolvedFields?: Record<string, string>;
	    // Go type: time
	    startsAt: any;
	    // Go type: time
	    updatedAt: any;
	    generatorURL: string;
	    silencedBy: string[];
	    silences?: SilenceInfo[];
	    inhibitedBy: string[];
	    receivers: string[];
	    hiddenBy?: string[];
	
	    static createFrom(source: any = {}) {
	        return new Alert(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.source = source["source"];
	        this.sourceType = source["sourceType"];
	        this.name = source["name"];
	        this.severity = source["severity"];
	        this.state = source["state"];
	        this.labels = source["labels"];
	        this.annotations = source["annotations"];
	        this.resolvedLabels = source["resolvedLabels"];
	        this.resolvedAnnotations = source["resolvedAnnotations"];
	        this.resolvedFields = source["resolvedFields"];
	        this.startsAt = this.convertValues(source["startsAt"], null);
	        this.updatedAt = this.convertValues(source["updatedAt"], null);
	        this.generatorURL = source["generatorURL"];
	        this.silencedBy = source["silencedBy"];
	        this.silences = this.convertValues(source["silences"], SilenceInfo);
	        this.inhibitedBy = source["inhibitedBy"];
	        this.receivers = source["receivers"];
	        this.hiddenBy = source["hiddenBy"];
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
	
	export class OnCallUser {
	    name: string;
	    email: string;
	
	    static createFrom(source: any = {}) {
	        return new OnCallUser(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.email = source["email"];
	    }
	}
	export class OnCallStatus {
	    source: string;
	    scheduleID: string;
	    scheduleName: string;
	    teamName?: string;
	    users: OnCallUser[];
	    // Go type: time
	    lastUpdated: any;
	
	    static createFrom(source: any = {}) {
	        return new OnCallStatus(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.source = source["source"];
	        this.scheduleID = source["scheduleID"];
	        this.scheduleName = source["scheduleName"];
	        this.teamName = source["teamName"];
	        this.users = this.convertValues(source["users"], OnCallUser);
	        this.lastUpdated = this.convertValues(source["lastUpdated"], null);
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
	
	
	export class SourceHealth {
	    source: string;
	    ok: boolean;
	    pending: boolean;
	    // Go type: time
	    lastPoll: any;
	    lastError?: string;
	    consecFails: number;
	
	    static createFrom(source: any = {}) {
	        return new SourceHealth(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.source = source["source"];
	        this.ok = source["ok"];
	        this.pending = source["pending"];
	        this.lastPoll = this.convertValues(source["lastPoll"], null);
	        this.lastError = source["lastError"];
	        this.consecFails = source["consecFails"];
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

}

export namespace provider {

	export class OIDCSessionInfo {
	    source: string;
	    configured: boolean;
	    active: boolean;
	    saved: boolean;
	    persistenceEnabled: boolean;
	    storageBackend?: string;
	    storageError?: string;

	    static createFrom(source: any = {}) {
	        return new OIDCSessionInfo(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.source = source["source"];
	        this.configured = source["configured"];
	        this.active = source["active"];
	        this.saved = source["saved"];
	        this.persistenceEnabled = source["persistenceEnabled"];
	        this.storageBackend = source["storageBackend"];
	        this.storageError = source["storageError"];
	    }
	}

}
