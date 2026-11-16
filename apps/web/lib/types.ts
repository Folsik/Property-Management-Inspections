export type Role="owner"|"manager"|"inspector";
export type Organization={id:string;name:string;role:Role};
export type Property={id:string;name:string;address:Record<string,string>};
export type InspectionType="move_in"|"move_out"|"turnover"|"maintenance";
export type InspectionStatus="draft"|"in_progress"|"completed"|"cancelled";
export type Inspection={id:string;property_id:string;property_name:string;type:InspectionType;status:InspectionStatus;created_at:string;open_items:number;total_items:number};
export type ItemStatus="open"|"in_progress"|"resolved";
export type InspectionItem={id:string;client_id:string;title:string;notes:string;severity:"low"|"normal"|"high"|"critical";status:ItemStatus;created_at:string};
export type Member={id:string;phone:string;full_name:string;role:Role;joined_at:string};
export type Media={id:string;mime_type:string;size_bytes:number;state:string;created_at:string};
export type Job={id:string;kind:string;state:string;attempts:number;created_at:string};
export type Profile={id:string;phone:string;full_name:string};
export type Collection<T>={items:T[]};
export const typeNames:Record<InspectionType,string>={move_in:"Move-in",move_out:"Move-out",turnover:"Turnover",maintenance:"Maintenance"};
export const statusNames:Record<InspectionStatus,string>={draft:"Draft",in_progress:"In progress",completed:"Completed",cancelled:"Cancelled"};

export type Contact={id:string;name:string;category:"owner"|"tenant"|"vendor"|"other";email:string;phone:string;notes:string;property_id:string|null;created_at:string};
