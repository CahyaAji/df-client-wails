export namespace main {
	
	export class UTMLocation {
	    zone: string;
	    easting: string;
	    northing: string;
	    co: string;
	
	    static createFrom(source: any = {}) {
	        return new UTMLocation(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.zone = source["zone"];
	        this.easting = source["easting"];
	        this.northing = source["northing"];
	        this.co = source["co"];
	    }
	}
	export class GPSLocation {
	    lat: number;
	    lng: number;
	
	    static createFrom(source: any = {}) {
	        return new GPSLocation(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.lat = source["lat"];
	        this.lng = source["lng"];
	    }
	}
	export class AppConfig {
	    map_key: string;
	    compass_offset: number;
	    gps_location: GPSLocation;
	    utm_location: UTMLocation;
	
	    static createFrom(source: any = {}) {
	        return new AppConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.map_key = source["map_key"];
	        this.compass_offset = source["compass_offset"];
	        this.gps_location = this.convertValues(source["gps_location"], GPSLocation);
	        this.utm_location = this.convertValues(source["utm_location"], UTMLocation);
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
	export class Bookmark {
	    id: number;
	    title: string;
	    style: string;
	    min_zoom: number;
	    max_zoom: number;
	    north: number;
	    south: number;
	    east: number;
	    west: number;
	    center_lat: number;
	    center_lng: number;
	
	    static createFrom(source: any = {}) {
	        return new Bookmark(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.title = source["title"];
	        this.style = source["style"];
	        this.min_zoom = source["min_zoom"];
	        this.max_zoom = source["max_zoom"];
	        this.north = source["north"];
	        this.south = source["south"];
	        this.east = source["east"];
	        this.west = source["west"];
	        this.center_lat = source["center_lat"];
	        this.center_lng = source["center_lng"];
	    }
	}
	

}

