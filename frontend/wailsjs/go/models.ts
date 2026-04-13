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
	
	export class AuthConfig {
	    Type: string;
	    Username: string;
	    Password: string;
	    Token: string;
	
	    static createFrom(source: any = {}) {
	        return new AuthConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.Type = source["Type"];
	        this.Username = source["Username"];
	        this.Password = source["Password"];
	        this.Token = source["Token"];
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
	export class BetterStackConfig {
	    OnCallSchedule: string;
	    TeamName: string;
	    TeamID: string;
	
	    static createFrom(source: any = {}) {
	        return new BetterStackConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.OnCallSchedule = source["OnCallSchedule"];
	        this.TeamName = source["TeamName"];
	        this.TeamID = source["TeamID"];
	    }
	}
	export class UIConfig {
	    theme: string;
	    popup_width: number;
	    popup_height: number;
	    show_resolved: boolean;
	    show_silenced: boolean;
	    default_created_by: string;
	    idle_image: string;
	
	    static createFrom(source: any = {}) {
	        return new UIConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.theme = source["theme"];
	        this.popup_width = source["popup_width"];
	        this.popup_height = source["popup_height"];
	        this.show_resolved = source["show_resolved"];
	        this.show_silenced = source["show_silenced"];
	        this.default_created_by = source["default_created_by"];
	        this.idle_image = source["idle_image"];
	    }
	}
	export class ResolverConfig {
	    Name: string;
	    Field: string;
	    Command: string;
	    Args: string[];
	    Env: Record<string, string>;
	    Timeout: number;
	    CacheTTL: number;
	
	    static createFrom(source: any = {}) {
	        return new ResolverConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.Name = source["Name"];
	        this.Field = source["Field"];
	        this.Command = source["Command"];
	        this.Args = source["Args"];
	        this.Env = source["Env"];
	        this.Timeout = source["Timeout"];
	        this.CacheTTL = source["CacheTTL"];
	    }
	}
	export class NotificationsConfig {
	    Enabled: boolean;
	    OnNew: boolean;
	    OnResolved: boolean;
	    BatchThreshold: number;
	
	    static createFrom(source: any = {}) {
	        return new NotificationsConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.Enabled = source["Enabled"];
	        this.OnNew = source["OnNew"];
	        this.OnResolved = source["OnResolved"];
	        this.BatchThreshold = source["BatchThreshold"];
	    }
	}
	export class SoundOverrides {
	    Critical?: SoundEntry;
	    Warning?: SoundEntry;
	    Info?: SoundEntry;
	
	    static createFrom(source: any = {}) {
	        return new SoundOverrides(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.Critical = this.convertValues(source["Critical"], SoundEntry);
	        this.Warning = this.convertValues(source["Warning"], SoundEntry);
	        this.Info = this.convertValues(source["Info"], SoundEntry);
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
	export class SoundEntry {
	    File: string;
	    Repeat: number;
	    Interval: number;
	
	    static createFrom(source: any = {}) {
	        return new SoundEntry(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.File = source["File"];
	        this.Repeat = source["Repeat"];
	        this.Interval = source["Interval"];
	    }
	}
	export class SoundsConfig {
	    Enabled: boolean;
	    Critical?: SoundEntry;
	    Warning?: SoundEntry;
	    Info?: SoundEntry;
	    Sources: Record<string, SoundOverrides>;
	
	    static createFrom(source: any = {}) {
	        return new SoundsConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.Enabled = source["Enabled"];
	        this.Critical = this.convertValues(source["Critical"], SoundEntry);
	        this.Warning = this.convertValues(source["Warning"], SoundEntry);
	        this.Info = this.convertValues(source["Info"], SoundEntry);
	        this.Sources = this.convertValues(source["Sources"], SoundOverrides, true);
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
	export class DisplayConfig {
	    visible_labels: string[];
	    visible_annotations: string[];
	    subtitle_annotations: string[];
	    group_by: string[];
	    group_by_override_key_mode: string;
	    group_by_overrides: Record<string, Array<string>>;
	    priority: DisplayPriority;
	    badges: BadgeRule[];
	
	    static createFrom(source: any = {}) {
	        return new DisplayConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.visible_labels = source["visible_labels"];
	        this.visible_annotations = source["visible_annotations"];
	        this.subtitle_annotations = source["subtitle_annotations"];
	        this.group_by = source["group_by"];
	        this.group_by_override_key_mode = source["group_by_override_key_mode"];
	        this.group_by_overrides = source["group_by_overrides"];
	        this.priority = this.convertValues(source["priority"], DisplayPriority);
	        this.badges = this.convertValues(source["badges"], BadgeRule);
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
	export class SeverityLevel {
	    name: string;
	    color: string;
	    aliases: string[];
	
	    static createFrom(source: any = {}) {
	        return new SeverityLevel(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.color = source["color"];
	        this.aliases = source["aliases"];
	    }
	}
	export class SeverityConfig {
	    default: string;
	    levels: SeverityLevel[];
	
	    static createFrom(source: any = {}) {
	        return new SeverityConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.default = source["default"];
	        this.levels = this.convertValues(source["levels"], SeverityLevel);
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
	export class SourceConfig {
	    Name: string;
	    Type: string;
	    URL: string;
	    Auth: AuthConfig;
	    PollInterval: number;
	    Filters: string[];
	    SeverityLabel: string;
	    BetterStack: BetterStackConfig;
	
	    static createFrom(source: any = {}) {
	        return new SourceConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.Name = source["Name"];
	        this.Type = source["Type"];
	        this.URL = source["URL"];
	        this.Auth = this.convertValues(source["Auth"], AuthConfig);
	        this.PollInterval = source["PollInterval"];
	        this.Filters = source["Filters"];
	        this.SeverityLabel = source["SeverityLabel"];
	        this.BetterStack = this.convertValues(source["BetterStack"], BetterStackConfig);
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
	export class Config {
	    Sources: SourceConfig[];
	    Severities: SeverityConfig;
	    Display: DisplayConfig;
	    Sounds: SoundsConfig;
	    Notifications: NotificationsConfig;
	    Actions: ActionConfig[];
	    Resolvers: ResolverConfig[];
	    UI: UIConfig;
	
	    static createFrom(source: any = {}) {
	        return new Config(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.Sources = this.convertValues(source["Sources"], SourceConfig);
	        this.Severities = this.convertValues(source["Severities"], SeverityConfig);
	        this.Display = this.convertValues(source["Display"], DisplayConfig);
	        this.Sounds = this.convertValues(source["Sounds"], SoundsConfig);
	        this.Notifications = this.convertValues(source["Notifications"], NotificationsConfig);
	        this.Actions = this.convertValues(source["Actions"], ActionConfig);
	        this.Resolvers = this.convertValues(source["Resolvers"], ResolverConfig);
	        this.UI = this.convertValues(source["UI"], UIConfig);
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
	export class NormalizedDisplayConfig {
	    visible_labels: string[];
	    visible_annotations: string[];
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
	        this.visible_labels = source["visible_labels"];
	        this.visible_annotations = source["visible_annotations"];
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
	
	
	
	
	
	
	
	
	
	

}

export namespace model {
	
	export class SilenceInfo {
	    id: string;
	    createdBy: string;
	    comment: string;
	    // Go type: time
	    startsAt: any;
	    // Go type: time
	    endsAt: any;
	
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
	export class Diff {
	    new: Alert[];
	    resolved: Alert[];
	    changed: Alert[];
	
	    static createFrom(source: any = {}) {
	        return new Diff(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.new = this.convertValues(source["new"], Alert);
	        this.resolved = this.convertValues(source["resolved"], Alert);
	        this.changed = this.convertValues(source["changed"], Alert);
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

