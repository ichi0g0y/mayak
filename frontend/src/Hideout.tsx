import { Language, translate } from './i18n'

export type HideoutEvent={kind:string;action:string;areaType:number;stationName:string;mode:string;accountId:string;profileId:string;occurredAt:string;status:string;historical:boolean;source:string}
type Level={id:string;level:number;constructionTime:number;itemRequirements:{item:string;name:string;count:number;attributes:{foundInRaid:boolean}}[];stationLevelRequirements:{station:string;name:string;level:number}[];traderRequirements:{trader:string;name:string;level:number}[];skillRequirements:{skill:string;level:number}[]}
export type HideoutStatus={state:string;mode:string;accountId:string;profileId:string;completed:number;total:number;lastError:string;events:HideoutEvent[];stations:{id:string;name:string;areaType:number;currentLevel:number;completedLevels:number[];nextLevel?:Level|null}[]}

export function hideoutMessage(event:HideoutEvent,language:Language){
 const t=(key:Parameters<typeof translate>[1])=>translate(language,key)
 const station=event.stationName||(event.areaType>=0?`${t('hideoutArea')} ${event.areaType}`:t('hideoutTitle'))
 const keys={'upgrade-failed':'hideoutUpgradeFailed','queue-failed':'hideoutQueueFailed','request-failed':'hideoutRequestFailed','metadata-request':'hideoutMetadataRequest','metadata-response':'hideoutMetadataResponse','unknown-action':'hideoutUnknownAction'} as const
 return `${station} · ${t(keys[event.kind as keyof typeof keys]??'hideoutUnknownAction')}`
}
