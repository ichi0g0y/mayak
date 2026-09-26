import React from 'react'
import { Card, CardContent } from './components/ui/card'

export function Metric({icon,label,value}:{icon:React.ReactNode;label:string;value:string}){return <Card><CardContent className="metric"><span>{icon}</span><div><small>{label}</small><strong>{value}</strong></div></CardContent></Card>}
