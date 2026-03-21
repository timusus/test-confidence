package com.example

import org.junit.Test
import org.junit.Ignore
import java.lang.reflect.Field

class AntiPatternTest {
    @Test
    fun `test with sleep`() {
        doSomething()
        Thread.sleep(1000)
        assertResult()
    }

    @Test
    @Ignore
    fun `ignored test`() {
        // nothing here
    }

    @Test
    fun `empty test`() {
    }

    @Test
    fun `test with conditional`() {
        val result = compute()
        if (result > 0) {
            assertEquals(expected, result)
        }
    }

    @Test
    fun `normal test`() {
        val result = sut.doThing()
        assertEquals(expected, result)
    }
}
