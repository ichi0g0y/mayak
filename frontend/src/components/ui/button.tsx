import * as React from 'react'
import { cva, type VariantProps } from 'class-variance-authority'
import { cn } from '../../lib/utils'

const buttonVariants = cva('button', { variants: { variant: { default:'button-default', secondary:'button-secondary', ghost:'button-ghost' }, size:{ default:'button-md', sm:'button-sm' } }, defaultVariants:{variant:'default',size:'default'} })
export interface ButtonProps extends React.ButtonHTMLAttributes<HTMLButtonElement>, VariantProps<typeof buttonVariants> {}
export const Button = React.forwardRef<HTMLButtonElement,ButtonProps>(({className,variant,size,...props},ref)=><button ref={ref} className={cn(buttonVariants({variant,size}),className)} {...props}/>)
Button.displayName='Button'
