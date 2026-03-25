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
	
	    static createFrom(source: any = {}) {
	        return new AuthConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.Type = source["Type"];
	        this.Username = source["Username"];
	        this.Password = source["Password"];
	    }
	}
	export class UIConfig {
	    Theme: string;
	    PopupWidth: number;
	    PopupHeight: number;
	    ShowResolved: boolean;
	    ShowSilenced: boolean;
	
	    static createFrom(source: any = {}) {
	        return new UIConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.Theme = source["Theme"];
	        this.PopupWidth = source["PopupWidth"];
	        this.PopupHeight = source["PopupHeight"];
	        this.ShowResolved = source["ShowResolved"];
	        this.ShowSilenced = source["ShowSilenced"];
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
	export class DisplayConfig {
	    VisibleLabels: string[];
	    VisibleAnnotations: string[];
	    GroupBy: string[];
	    SortBy: string;
	
	    static createFrom(source: any = {}) {
	        return new DisplayConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.VisibleLabels = source["VisibleLabels"];
	        this.VisibleAnnotations = source["VisibleAnnotations"];
	        this.GroupBy = source["GroupBy"];
	        this.SortBy = source["SortBy"];
	    }
	}
	export class SourceConfig {
	    Name: string;
	    Type: string;
	    URL: string;
	    Auth: AuthConfig;
	    PollInterval: number;
	    Filters: string[];
	
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
	    Display: DisplayConfig;
	    Sounds: SoundsConfig;
	    Notifications: NotificationsConfig;
	    Actions: ActionConfig[];
	    UI: UIConfig;
	
	    static createFrom(source: any = {}) {
	        return new Config(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.Sources = this.convertValues(source["Sources"], SourceConfig);
	        this.Display = this.convertValues(source["Display"], DisplayConfig);
	        this.Sounds = this.convertValues(source["Sounds"], SoundsConfig);
	        this.Notifications = this.convertValues(source["Notifications"], NotificationsConfig);
	        this.Actions = this.convertValues(source["Actions"], ActionConfig);
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
	
	
	
	
	
	

}

export namespace model {
	
	export class Alert {
	    id: string;
	    source: string;
	    sourceType: string;
	    name: string;
	    severity: string;
	    state: string;
	    labels: Record<string, string>;
	    annotations: Record<string, string>;
	    // Go type: time
	    startsAt: any;
	    // Go type: time
	    updatedAt: any;
	    generatorURL: string;
	    silencedBy: string[];
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
	        this.startsAt = this.convertValues(source["startsAt"], null);
	        this.updatedAt = this.convertValues(source["updatedAt"], null);
	        this.generatorURL = source["generatorURL"];
	        this.silencedBy = source["silencedBy"];
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
	export class SeverityCounts {
	    critical: number;
	    warning: number;
	    info: number;
	
	    static createFrom(source: any = {}) {
	        return new SeverityCounts(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.critical = source["critical"];
	        this.warning = source["warning"];
	        this.info = source["info"];
	    }
	}

}

