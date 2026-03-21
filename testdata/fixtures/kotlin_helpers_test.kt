package com.example

import io.mockk.every
import io.mockk.verify
import io.mockk.coEvery
import io.mockk.coVerify
import org.junit.Test
import kotlinx.coroutines.test.runTest

class HelperTestCases {
    @Test
    fun `verify calls`() {
        verify { repo.save(user) }
        coVerify { repo.saveAsync(user) }
    }

    @Test
    fun `stub calls`() {
        every { repo.getUser() } returns user
        coEvery { repo.getUserAsync() } returns user
    }

    @Test
    fun `assertion calls`() {
        assertThat(result).isEqualTo(expected)
        assertEquals(expected, actual)
        assertTrue(flag)
        assertNotNull(value)
        assertThrows<Exception> { sut.doThing() }
    }

    @Test
    fun `mixed calls`() {
        every { repo.getUser() } returns user
        val result = viewModel.loadUser()
        assertThat(result).isEqualTo(user)
        verify { repo.getUser() }
    }

    @Test
    fun `delay in runTest`() {
        runTest {
            delay(100)
            val result = sut.doThing()
        }
    }
}
